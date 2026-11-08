// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// FootprintDoorOpeningsAgainstScan asks every footprint door of enc for its
// opening and compares it with the whole-floor scan
// ([wholeFloorOpening]): how many doors were asked, and the ids of those
// whose answers differ. A test-only window for the keep fixtures, which the
// internal tests cannot build (rpg-project#879 S9).
func FootprintDoorOpeningsAgainstScan(enc *Encounter) (asked int, crossings int, differ []DoorID) {
	for _, d := range enc.doors {
		if d.placement == nil {
			continue
		}
		asked++
		got := enc.doorOpening(d.id)
		crossings += len(got)
		want := wholeFloorOpening(enc.field, d)
		if len(got) != len(want) {
			differ = append(differ, d.id)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				differ = append(differ, d.id)
				break
			}
		}
	}

	return asked, crossings, differ
}
