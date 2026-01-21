# Property-Based Testing Guidance

**Status:** Authoritative
**Applies to:** All test creation decisions
**Audience:** Kiro and developers

---

## 1. Purpose

This document defines **when property-based testing (PBT) is appropriate** versus when simpler unit tests suffice.

The goal is to:
- Avoid over-engineering test suites
- Reserve PBT for cases where it genuinely adds value
- Prefer simpler, more readable tests when they provide equivalent confidence

This document is **normative**.
Kiro MUST consult this document before proposing property-based tests.

---

## 2. Core Principle

> **Property tests are a tool, not a default.**

Unit tests with well-chosen examples are often sufficient and more maintainable.

---

## 3. When Property Tests ARE Warranted

Property-based testing SHOULD be used when:

1. **Mathematical invariants exist**
   - Sorting algorithms (output is sorted, same elements)
   - Calculations with known algebraic properties
   - Encoding/decoding roundtrips

2. **Complex input spaces with non-obvious edge cases**
   - Parsers and serializers
   - Data transformation pipelines
   - Input validation with many boundary conditions

3. **Business rules with universal properties**
   - "Rankings must be unique and contiguous"
   - "Totals must equal sum of parts"
   - "Status transitions must follow state machine"

4. **Existing bugs suggest missed edge cases**
   - When a bug reveals the input space wasn't adequately explored
   - When manual enumeration of cases is impractical

---

## 4. When Property Tests Are NOT Warranted

Property-based testing SHOULD NOT be used for:

1. **UI component fixes**
   - Display formatting
   - Layout adjustments
   - Styling changes

2. **Simple CRUD operations**
   - Basic create/read/update/delete
   - Straightforward data mapping

3. **Integration glue code**
   - Wiring between components
   - Configuration handling

4. **Cases where examples are clearer**
   - When 3-5 specific examples fully cover the behavior
   - When the "property" would just restate the implementation

5. **Low-risk, easily-observable changes**
   - Changes where failures would be immediately obvious
   - Changes with low blast radius

---

## 5. Decision Framework

Before proposing a property test, answer:

1. **What universal property would this test verify?**
   - If you can't articulate a clear property, use unit tests

2. **Would 5 well-chosen examples provide equivalent confidence?**
   - If yes, prefer the examples

3. **Is the input space genuinely complex?**
   - If inputs are simple or bounded, examples suffice

4. **Does this logic have mathematical or algebraic properties?**
   - If no, property tests likely add complexity without value

---

## 6. Final Rule

> **Prefer the simplest test that provides confidence.**
> **Property tests are for invariants, not for everything.**
> **When in doubt, write unit tests with good examples.**
