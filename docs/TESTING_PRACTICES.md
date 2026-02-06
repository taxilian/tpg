# Test Best Practices

## Core Principle: Test Behavior, Not Implementation

Tests should verify **what** your code does, not **how** it does it. If you can refactor the implementation without changing tests, you're testing the right things.

## The Golden Rules

1. **A test should fail only when behavior changes**
2. **Tests are documentation - they show how to use your code**
3. **Each test should verify one specific behavior**
4. **Test names should describe user-visible outcomes**

## What TO Test ✅

### 1. **Public API Contracts**
```python
# GOOD: Testing the contract
def test_calculate_total_includes_tax():
    order = create_order(subtotal=100, tax_rate=0.08)
    assert order.calculate_total() == 108
```

### 2. **Edge Cases and Error Conditions**
```java
// GOOD: Testing boundary behavior
@Test
void shouldRejectNegativeQuantities() {
    assertThrows(ValidationException.class, 
        () -> new OrderItem("widget", -1));
}
```

### 3. **Business Logic and Rules**
```javascript
// GOOD: Testing business requirements
test('applies bulk discount for orders over $1000', () => {
  const order = createOrder({ subtotal: 1200 });
  expect(order.getDiscount()).toBe(120); // 10% discount
});
```

### 4. **Integration Between Components**
```ruby
# GOOD: Testing components work together
it "sends email when order is completed" do
  order = create(:order)
  order.complete!
  
  expect(EmailService.sent_emails).to include(
    have_attributes(to: order.customer_email, subject: /completed/)
  )
end
```

## What NOT to Test ❌

### 1. **Default Values That Haven't Changed**
```python
# BAD: Testing that defaults are still defaults
def test_default_timeout():
    config = Config()
    assert config.timeout == 5000  # Worthless!
```

### 2. **Simple Property Assignment**
```java
// BAD: Testing the language works
@Test
void testSetName() {
    user.setName("John");
    assertEquals("John", user.getName()); // Pointless!
}
```

### 3. **Private Methods Directly**
```javascript
// BAD: Testing internals
test('private calculate method', () => {
  // Don't test private methods!
  const result = service._calculateDiscount(100, 0.1);
  expect(result).toBe(10);
});
```

### 4. **Exact Implementation Details**
```ruby
# BAD: Testing HOW not WHAT
it "calls the repository save method" do
  expect(repository).to receive(:save).with(user)
  service.create_user(user_data)
end
```

### 5. **Framework Behavior**
```python
# BAD: Testing that the framework works
def test_django_model_save():
    user = User(name="test")
    user.save()
    assert user.id is not None  # Django's job, not yours
```

## Test Structure

### Arrange-Act-Assert (AAA)
```typescript
test('should process refund for returned items', () => {
  // Arrange: Set up the test scenario
  const order = createCompletedOrder({ total: 100 });
  const returnRequest = { orderId: order.id, reason: 'defective' };
  
  // Act: Execute the behavior
  const refund = processReturn(returnRequest);
  
  // Assert: Verify the outcome
  expect(refund.amount).toBe(100);
  expect(order.status).toBe('refunded');
});
```

### Given-When-Then (BDD Style)
```ruby
describe "Order cancellation" do
  context "when order hasn't shipped" do
    it "refunds the full amount" do
      # Given
      order = create(:order, status: 'pending', total: 50)
      
      # When
      result = order.cancel
      
      # Then
      expect(result.refund_amount).to eq(50)
      expect(order.status).to eq('cancelled')
    end
  end
end
```

## Common Anti-Patterns and Fixes

### 1. **Testing Too Much in One Test**
```python
# BAD: Multiple behaviors in one test
def test_user_creation():
    user = User("john", "john@example.com")
    assert user.name == "john"
    assert user.email == "john@example.com"
    assert user.is_active == True
    assert user.created_at is not None
    assert len(user.get_permissions()) == 3

# GOOD: Separate tests for separate behaviors
def test_user_starts_active():
    user = User("john", "john@example.com")
    assert user.is_active == True

def test_new_user_has_default_permissions():
    user = User("john", "john@example.com")
    assert user.has_permission('read')
    assert user.has_permission('write')
    assert not user.has_permission('admin')
```

### 2. **Brittle Time-Based Tests**
```javascript
// BAD: Depends on real time
test('expires after one hour', async () => {
  const token = createToken();
  await sleep(3600000); // Wait an actual hour!
  expect(isExpired(token)).toBe(true);
});

// GOOD: Control time in tests
test('expires after one hour', () => {
  const clock = mockTime();
  const token = createToken();
  
  clock.advance(hours(1));
  
  expect(isExpired(token)).toBe(true);
});
```

### 3. **Over-Mocking**
```java
// BAD: Everything is mocked
@Test
void testOrderService() {
    Database mockDb = mock(Database.class);
    Logger mockLogger = mock(Logger.class);
    EmailService mockEmail = mock(EmailService.class);
    PaymentService mockPayment = mock(PaymentService.class);
    
    // This tests nothing real!
}

// GOOD: Mock only external boundaries
@Test 
void testOrderProcessing() {
    // Real implementations
    Database db = new InMemoryDatabase();
    Logger logger = new TestLogger();
    
    // Mock only external services
    EmailService mockEmail = mock(EmailService.class);
    PaymentGateway mockPayment = mock(PaymentGateway.class);
    
    OrderService service = new OrderService(db, logger, mockEmail, mockPayment);
    // Now we're testing real behavior
}
```

## Quick Reference: Test Smells

- 🚫 **Test changes when refactoring** → Testing implementation
- 🚫 **Test name includes method names** → Not describing behavior  
- 🚫 **Test has many assertions** → Testing too much at once
- 🚫 **Test uses private access** → Breaking encapsulation
- 🚫 **Test verifies mock interactions** → Not testing outcomes
- 🚫 **Test is slow** → Probably doing real I/O
- 🚫 **Test needs specific order** → Hidden dependencies

## The One Question That Matters

Before writing any test, ask: **"What would a user notice if this test failed?"**

If the answer is "nothing" or "they wouldn't care", don't write the test.

---

*Remember: Tests are not about coverage metrics. They're about confidence that your system works correctly for its users.*
