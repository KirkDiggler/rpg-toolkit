# Composable maintenance — acceptance and evidence boundaries

This is the verification brief for [the approved contracts](README.md), not a checked implementation plan or a record of passing new tests. Exact test files, names, mutants and dependency pins belong to the implementation plan derived after design agreement. Existing baseline receipts live on [S3 #706](https://github.com/KirkDiggler/rpg-project/issues/706).

## Named scenarios

| Scenario | Assertion | Owning proof |
|---|---|---|
| Independent parts share a beat | Two nonterminal parts each update once; their state keys cannot collide. One first-match list cannot silently suppress the other. | Generic assembly/machine tests |
| Part and application termination differ | A local part can complete without ending unrelated parts; an application-scoped terminal request ends exactly its lifetime once. | Generic assembly/lifetime tests |
| Broken construction/restore | Duplicate keys, undeclared bag values, unknown kinds, missing snapshots, invalid terminal maintenance and wrong lifetime roles fail by name before attachment. | Generic + root registry tests |
| Partial attach | A later part's attach failure unwinds all successful earlier attachment; no subscription survives. | Generic assembly tests |
| Teardown is not expiry | End an interaction, serialize, reload and attach; no source is spent, time advanced or gameplay removal published by teardown. | Generic + resolution persistence tests |
| Honest dice | Information returns an unresolved signed d4 with zero roller calls; the native attack/save path rolls selected contributions once. Scalar-only consumers refuse dice results. | Generic amount + root selector + resolution tests |
| Two Bless sources and Bane | Both Bless instances persist, one Bless die is selected, and Bane subtracts independently. Removing one Bless source exposes the other without recreating it. | Root, resolution and session |
| Self-maintained Bless | The caster's Bless can contribute to the concentration save; only after failure does linked removal occur. | Real damage/follow-up/save path |
| One hold, several recipients | One damage fact produces one concentration check, not one per recipient. Pass preserves all links; failure removes the exact linked recipients and coordinator. | Real damage/follow-up path, including reload |
| Independent sources survive | End application A while application B affects the same recipient; no B attachment, contribution or resource changes. | Exact-address and link-ownership regressions |
| Native recast and refusal | A refused action drops nothing and spends nothing; a paid concentration replacement uses the existing ordering and does not confuse old/new application identity. | Native cast/door tests |
| Zero delivered recipients | Paid zero-delivery behavior and prior-hold displacement match the current native path; no false last-child event is invented at creation. | Native cast regression |
| Last child, self child, independent child | Removing one recipient preserves the remaining members; last-member loss follows the authored end policy; independent-duration payloads are outside the group. Self-recipient and coordinator are separate attachments. | Root + resolution |
| Duration/rest/down/combat end | Grace and subsequent caster turn-end count survive reload; current end reasons remain; zero HP ends without a save; rest/departure do not orphan other holders. | Root + resolution + session |
| Area concentration | An area-only hold with zero condition children retains the existing area close path; geometry is not moved into effects. | Existing native area regression adapted to the new maintenance protocol |
| Resistance composition | Concentration + existing physical resistance protects another holder; the same payload works without concentration and with an optional duration. No Rage bonus, class budget or activity rule appears. | Generic/root executable example |
| Resistance and native Raging coexist | Ending the maintained resistance does not remove the independently active Raging instance or its contribution. | Root + real incoming-damage regression |
| Linked integrity | Missing holder, foreign application link, competing parent claim or a stale member targeting a replacement is refused; duplicate legitimate removal is idempotent. | Link transport/preflight/departure tests |
| Shared concentration migration | Current spell callers use one maintenance protocol, not a mix of the old class and a new Bless-only engine. Non-migrated payload behavior remains unchanged. | Caller audit + owning native regression suites |
| Native information | Normally created caster and recipient see server-owned source, applicability and maintenance facts before/after cast, save, reload and end. Recipient-only views do not invent an unavailable countdown. | API contracts and bounded browser walk |

## Delivery boundary

The approved design leads to a checked plan and independently versioned provider/consumer slices. One nearest-go.mod module per toolkit PR, pushed development pins during integration, real CI tags during authorized merge-out. A data/contract requirement exposed by a real consumer is resolved at its owner; it is not reconstructed in web/API.

Required evidence is separated into generic tool tests, rulebook tests, real resolution/session tests, API persistence/projection and native browser observations. The operator's early look precedes one independent review round and the bounded walk. A specific later finding gets focused closure, not another whole-diff review campaign.

The normal native journey uses freshly created eligible characters, actual spell acquisition, targeting and resources. Two sources on one recipient and a multi-recipient hold are exercised. Dice-dependent branches may use named real-path tests rather than repeated live rolls; receipts state which outcomes were observed and which were test-backed.

The concentrated-resistance example is architectural/tool proof, not a new production spell, item acquisition route or studio feature. Non-spell presentation through spell-specific wire messages is not claimed. It must not be implemented by labeling an item or feature as a spell.

No automatic character resets, local Go replacement graph, hand-generated proto bindings, manual tags, unrelated stack changes or production promotion are included. R8 explicitly assumes fresh characters and one current path. There is no legacy-save reader, dual writer, normalization pass or parallel old concentration engine.

## Finish line for this design pass

- Shape approval remains recorded separately from detailed-contract approval.
- Walkthrough and four-section design name the interfaces, state owners, end/refusal behavior, consumer boundaries and costs.
- All current native concentration consumers are accounted for, including unchanged payloads and area/departure paths.
- Detailed contracts are approved together on the design PR, including fresh-character-only data and the non-spell native boundary.
- No implementation or completed native QA is claimed by publishing these documents.
