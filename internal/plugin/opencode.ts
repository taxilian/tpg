/**
 * TPG OpenCode Plugin
 *
 * Integrates tpg task management into OpenCode sessions:
 * - Injects `tpg prime` context into system prompt (fresh each time)
 * - Injects `tpg prime` context during context compaction
 * - Adds AGENT_ID and AGENT_TYPE env vars to all `tpg` bash commands
 * - Provides tools to inspect subagent sessions (check task status without full context)
 */

import type { Plugin } from "@opencode-ai/plugin"
import { z } from "zod"

export const TpgPlugin: Plugin = async ({ $, directory, client }) => {
  // Cache agent type per session to avoid repeated API calls
  const agentTypeCache = new Map<string, "primary" | "subagent">()

  /**
   * Determine if a session is a subagent (has a parent session) or primary.
   * Uses client.session.get() API to check for parentID.
   */
  async function getAgentType(sessionID: string): Promise<"primary" | "subagent"> {
    const cached = agentTypeCache.get(sessionID)
    if (cached) return cached

    let agentType: "primary" | "subagent" = "primary"
    
    try {
      // Use client.session.get() API
      const result = await client.session.get({ path: { id: sessionID } })
      const session = (result as any)?.data || result
      
      // Check for parentID - if present, this is a subagent
      if (session?.parentID != null) {
        agentType = "subagent"
      }
    } catch {
      // If API fails, default to primary
    }

    agentTypeCache.set(sessionID, agentType)
    return agentType
  }

  /**
   * Run tpg prime and return output, or undefined if unavailable.
   */
  async function getPrime(sessionID?: string, agentType?: string): Promise<string | undefined> {
    try {
      const env: Record<string, string> = {}
      if (sessionID) {
        env.AGENT_ID = sessionID
        env.AGENT_TYPE = agentType || "primary"
      }
      const result = await $`tpg prime`.cwd(directory).env(env).quiet()
      const output = result.text().trim()
      return output || undefined
    } catch {
      return undefined
    }
  }

  return {
    /**
     * Inject fresh tpg prime context into system prompt.
     * Only injects when sessionID is available (skips agent config generation).
     */
    "experimental.chat.system.transform": async (input, output) => {
      // Skip if no sessionID (e.g., agent config generation doesn't need tpg context)
      if (!input.sessionID) return

      const agentType = await getAgentType(input.sessionID)
      const prime = await getPrime(input.sessionID, agentType)
      if (prime) {
        output.system.push(prime)
      }
    },

    /**
     * During context compaction, inject fresh tpg prime context.
     */
    "experimental.session.compacting": async (input, output) => {
      const agentType = await getAgentType(input.sessionID)
      const prime = await getPrime(input.sessionID, agentType)
      if (prime) {
        output.context.push(prime)
      }
    },

    /**
     * Before bash tool execution, inject AGENT_ID and AGENT_TYPE
     * environment variables into tpg commands.
     */
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "bash") return

      const cmd = output.args?.command
      if (typeof cmd !== "string") return

      // Only modify tpg commands
      if (!/(?:^|&&|\|\||[;|])\s*tpg(?:\s|$)/.test(cmd)) return

      const sessionID = input.sessionID
      const agentType = await getAgentType(sessionID)

      output.args.command = `AGENT_ID="${sessionID}" AGENT_TYPE="${agentType}" ${cmd}`
    },

    /**
     * Provide tools for inspecting subagent sessions.
     * Allows checking task status without pulling full context.
     */
    tool: {
      // Check if subagent exists and get its metadata
      inspect_subagent_metadata: {
        description: "Check if a subagent session exists and get its metadata (including message count estimate)",
        parameters: z.object({ subagentID: z.string() }),
        execute: async ({ subagentID }) => {
          try {
            const result = await client.session.get({ path: { id: subagentID } })
            const session = (result as any)?.data || result
            
            if (!session) {
              return { accessible: false, reason: "not_found" }
            }
            
            // Get message count for size estimate
            let messageCount = 0
            let sizeEstimate = "small"
            try {
              const messages = await client.session.messages({ sessionID: subagentID })
              messageCount = messages.length
              if (messageCount > 100) sizeEstimate = "large"
              else if (messageCount > 30) sizeEstimate = "medium"
            } catch {
              // Can't get message count, ignore
            }
            
            return {
              accessible: true,
              id: session.id,
              parentID: session.parentID,
              title: session.title,
              description: session.description,
              created: session.createdAt || session.time?.created,
              lastUpdate: session.updatedAt || session.time?.updated,
              directory: session.directory,
              isSubagent: session.parentID != null,
              messageCount,
              sizeEstimate
            }
          } catch {
            return { accessible: false, reason: "error_accessing" }
          }
        }
      },

      // List messages with only metadata (no content) - lightweight index
      list_subagent_messages: {
        description: "Get a lightweight index of messages in a subagent without full content",
        parameters: z.object({
          subagentID: z.string(),
          limit: z.number().optional().default(20)
        }),
        execute: async ({ subagentID, limit }) => {
          try {
            const messages = await client.session.messages({ sessionID: subagentID })
            const recent = messages.slice(-limit)
            
            return {
              totalMessages: messages.length,
              returned: recent.length,
              messages: recent.map(m => ({
                id: m.id,
                role: m.role,
                created: m.createdAt,
                toolCalls: m.toolCalls?.map((t: any) => t.tool) || [],
                hasErrors: m.toolResults?.some((r: any) => r.error) || false
              }))
            }
          } catch {
            return { error: "Failed to retrieve messages" }
          }
        }
      },

      // Get specific message content only when needed
      get_subagent_message: {
        description: "Retrieve full content of a specific message from a subagent",
        parameters: z.object({
          subagentID: z.string(),
          messageID: z.string()
        }),
        execute: async ({ subagentID, messageID }) => {
          try {
            const messages = await client.session.messages({ sessionID: subagentID })
            const message = messages.find(m => m.id === messageID)
            
            if (!message) {
              return { error: "Message not found" }
            }
            
            return {
              id: message.id,
              role: message.role,
              created: message.createdAt,
              content: message.content?.substring(0, 2000), // Truncate long content
              toolCalls: message.toolCalls,
              toolResults: message.toolResults?.map((r: any) => ({
                tool: r.tool,
                status: r.error ? "error" : "success",
                output: r.output?.substring(0, 1000) // Truncate
              }))
            }
          } catch {
            return { error: "Failed to retrieve message" }
          }
        }
      },

      // Get recent messages with content summaries for diagnosis
      get_subagent_recent: {
        description: "Get recent messages from a subagent with brief content summaries",
        parameters: z.object({
          subagentID: z.string(),
          count: z.number().optional().default(10)
        }),
        execute: async ({ subagentID, count }) => {
          try {
            const messages = await client.session.messages({ sessionID: subagentID })
            const recent = messages.slice(-count)
            
            return {
              totalMessages: messages.length,
              returned: recent.length,
              messages: recent.map(m => {
                // Build a brief summary
                let summary = ""
                if (m.role === "user") {
                  summary = m.content?.substring(0, 100) + (m.content?.length > 100 ? "..." : "") || "[no content]"
                } else if (m.role === "assistant") {
                  if (m.toolCalls?.length) {
                    summary = `Tool calls: ${m.toolCalls.map((t: any) => t.tool).join(", ")}`
                  } else {
                    summary = m.content?.substring(0, 100) + (m.content?.length > 100 ? "..." : "") || "[no content]"
                  }
                }
                
                return {
                  id: m.id,
                  role: m.role,
                  created: m.createdAt,
                  summary,
                  hasErrors: m.toolResults?.some((r: any) => r.error) || false
                }
              })
            }
          } catch {
            return { error: "Failed to retrieve messages" }
          }
        }
      },

      // Find error messages specifically
      find_subagent_errors: {
        description: "Find messages with tool errors in a subagent",
        parameters: z.object({
          subagentID: z.string(),
          limit: z.number().optional().default(5)
        }),
        execute: async ({ subagentID, limit }) => {
          try {
            const messages = await client.session.messages({ sessionID: subagentID })
            
            // Find messages with errors
            const errorMessages = messages
              .filter(m => m.toolResults?.some((r: any) => r.error))
              .slice(-limit)
            
            return {
              totalErrors: errorMessages.length,
              errors: errorMessages.map(m => {
                const failedTools = m.toolResults
                  ?.filter((r: any) => r.error)
                  ?.map((r: any) => ({
                    tool: r.tool,
                    error: r.error?.substring(0, 500)
                  })) || []
                
                return {
                  messageID: m.id,
                  created: m.createdAt,
                  failedTools,
                  contextBefore: messages[Math.max(0, messages.indexOf(m) - 1)]?.content?.substring(0, 200)
                }
              })
            }
          } catch {
            return { error: "Failed to search for errors" }
          }
        }
      },

      // Get a summary of what the subagent was working on
      summarize_subagent_work: {
        description: "Get a summary of what a subagent was working on based on its tool calls",
        parameters: z.object({
          subagentID: z.string()
        }),
        execute: async ({ subagentID }) => {
          try {
            const messages = await client.session.messages({ sessionID: subagentID })
            
            // Collect all tool calls
            const toolUsage: Record<string, number> = {}
            const fileReads: string[] = []
            const filesWritten: string[] = []
            const commands: string[] = []
            
            messages.forEach(m => {
              m.toolCalls?.forEach((t: any) => {
                toolUsage[t.tool] = (toolUsage[t.tool] || 0) + 1
                
                if (t.tool === "read" && t.args?.filePath) {
                  fileReads.push(t.args.filePath)
                }
                if (t.tool === "write" && t.args?.filePath) {
                  filesWritten.push(t.args.filePath)
                }
                if (t.tool === "bash" && t.args?.command) {
                  commands.push(t.args.command.split(" ")[0]) // Just the command name
                }
              })
            })
            
            return {
              totalMessages: messages.length,
              toolUsage: Object.entries(toolUsage)
                .sort((a, b) => b[1] - a[1])
                .slice(0, 10),
              filesRead: [...new Set(fileReads)].slice(-10),
              filesWritten: [...new Set(filesWritten)].slice(-10),
              commandsUsed: [...new Set(commands)].slice(-10)
            }
          } catch {
            return { error: "Failed to summarize work" }
          }
        }
      }
    }
  }
}

export default TpgPlugin
