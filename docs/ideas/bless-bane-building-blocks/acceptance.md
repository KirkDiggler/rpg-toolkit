# Observable proof for the first cut

These are proposed acceptance scenarios for the [design](design.md), not executed results or an implementation task list. The linked PR carries baseline observations and later evidence.

| Scenario | What must be observable |
|---|---|
| Construct Bless and Bane in Go | Definitions use typed AddToRoll and separately declared EndOn values. Errors return to the caller. No namedRollParts helper, handwritten JSON at those call sites or fallible initialization convenience is needed. |
| Author the same leaves as data | Go and JSON lower to equivalent canonical declarations and receive the same validation. Dice has one canonical amount location. |
| Build an unnamed variant | +1d6 on attacks only with a different explicit lifetime executes through the same implementation; no condition-name branch or new registry entry is introduced for its effect meaning. |
| Keep two Bless instances and one Bane | All three remain held with independent identities and lifetimes. The oldest applicable Bless and the Bane contribute; their sourced signed terms combine. |
| Use AllApplicable | Two independent applicable additive gifts both contribute. The additive meaning is unchanged; there is no special additive flag or fake per-instance group required to obtain this behavior. |
| Remove the selected Bless | The next eligible Bless contributes on the next query. Its identity, age and remaining lifetime are unchanged. |
| Expire a nonselected instance | It disappears under its own policy. Removing the winner later cannot revive the expired instance. |
| Select one effect of a multi-effect condition | Only the losing contribution is excluded. Another effect on that same condition still participates where applicable. |
| Save and reload a character | Before/after selection chooses the same source in the same order. A newly admitted instance follows restored survivors. |
| Save, reload and attach a monster | The carried condition order survives preparation/attachment and selects the same source. Subscription registration order is not an age authority. |
| Inspect versus resolve | Information and actual roll use the same selection decisions. A held applicable loser explains which older contribution was selected instead; it is not described as expired. |
| Count RNG use | Only selected dice are requested, exactly once; faces stay unsigned and provenance/sign survive the result breakdown. Querying applicability or selection rolls nothing. |
| Reject malformed input | Invalid dice, polarity, domains, keys/scopes and selection policies return errors, including malformed unselected definitions/candidates. The first cut does not pretend to implement fixed amounts or check offers. |
| Omit EndOn | The roll effect alone does not end on rest. Adding explicit recipient EndOn or coordinator maintenance supplies that behavior without changing roll mechanics. |
| Retain native application behavior | Normal Bless/Bane cast/payment, independent concentration cleanup, pending Bane saves, source identity and reload remain intact. Existing native Raging and other unchanged payloads still work. |

## Evidence boundaries

Pure selector and definition tests establish value contracts. Rulebook tests establish composition and keeper/roll integration. Consumer-module tests establish adoption of the provider graph. A native browser walk establishes what a player can acquire, read and observe; a scripted module fixture does not replace it. Exact native adoption and release slices belong to the checked implementation plan after agreement on the concrete API.
