# Guiding Bolt: development and acceptance

User direction: implement end to end using draft PRs and pushed provider pins.
Do not merge toolkit/API/web while developing. If an actual proto gap is found,
protos may merge so consumers can use its generated SDK. No go.work or local
replacements are needed. Keep one owning module per PR.

Source: Basic Rules (2014), Guiding Bolt, reviewed 2026-09-18 at
https://www.dndbeyond.com/spells/2133-guiding-bolt (complete legacy entry).
One action, level-one slot, 120 feet, verbal/somatic, ranged spell attack.
A hit deals 4d6 radiant damage and grants advantage on the next attack roll
against the target before the end of the caster's next turn. No concentration.
Printed scaling adds 1d6 per slot above first; higher-slot selection remains
outside the existing level-one contribution scope.

The condition belongs to the target and carries the originating caster for its
clock and attribution. Any attack roll against that target consumes it, even
if it misses or another source cancels advantage. A save or an attempt stopped
before the attack roll must not consume it. Other creatures' turns do not expire
it. The casting turn's end retains it; the caster's next turn end removes it.
Persist every clock transition and condition removal across reloads.

Current findings: AttackProfile already supports spell attacks and on-hit
conditions. CastProfile currently has saves/direct delivery only. Connect Cast
to the existing strike machinery, including post-roll pause/resume, rather than
create another attack resolver. Verify existing generic attack refs and sourced
roll facts can carry spell identity before proposing a protocol addition.

Root checkpoint: condition, loader/factory/display registry, caster-clock and
one-use tests, and explicit nested attack profile contract. Spell content and
native acquisition are not enabled yet. Resolution, encounter/session recording,
API integration and browser acceptance remain pending.

Acceptance: native Cleric acquisition; paid hit/miss/critical; radiant damage;
one-use advantage for another actor; miss consumption; save non-consumption;
caster-clock expiry across reload; no concentration replacement; Sanctuary gate
before the attack roll; live/Story/private refresh agree. Include post-roll offers
without duplicate rolls or payment. Bring up a complete local stack with assets,
authentication and exact versions, then verify the rendered scene before handoff.
