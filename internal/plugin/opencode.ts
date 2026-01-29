/**
 * TPG OpenCode Plugin
 *
 * Integrates tpg task management into OpenCode sessions:
 * - Injects `tpg prime` context into system prompt (fresh each time)
 * - Injects `tpg prime` context during context compaction
 * - Adds AGENT_ID and AGENT_TYPE env vars to all `tpg` bash commands
 */

import type { Plugin } from "@opencode-ai/plugin"

export const TpgPlugin: Plugin = async ({ $, directory, client }) => {
  // Cache agent type per session to avoid repeated API calls
  const agentTypeCache = new Map<string, "primary" | "subagent">()

  /**
   * Determine if a session is a subagent (has a parent session) or primary.
   */
  async function getAgentType(sessionID: string): Promise<"primary" | "subagent"> {
    const cached = agentTypeCache.get(sessionID)
    if (cached) return cached

    let agentType: "primary" | "subagent" = "primary"
    try {
      const result = await client.session.get({ sessionID })
      if ((result as any)?.data?.parentID || (result as any)?.parentID) {
        agentType = "subagent"
      }
    } catch {
      // Can't determine; default to primary
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
  }
}

export default TpgPlugin
