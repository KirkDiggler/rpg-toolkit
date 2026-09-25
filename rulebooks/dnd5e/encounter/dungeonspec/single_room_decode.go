package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"gopkg.in/yaml.v3"
)

// THE SHAPE WALK STOPS AT THE PRESENTATION (rpg-project#479, R3). The
// scene, the coordinate frame and the workspace are the World Builder's
// content: the editor's scalar bounds, its workspace presets, its frame
// vocabulary, its item and group caps and its parent/support graph were
// mirrored here and are gone, along with the typed shapes they judged. What
// this decoder still reads out of those three subtrees is the handful of
// values play depends on, and single_room_lowering.go names every one of them
// by path and by the gameplay fact that needs it.
//
// STRICTNESS OVER THE PRESENTATION BELONGS TO THE CODEC THAT OWNS IT — the
// web's, which already validates the draft it authors and refuses what it
// does not know. An unknown key inside `scene`, `coordinateFrame` or
// `workspace` therefore decodes here without a word. Strictness over the
// GAMEPLAY grammar is unchanged: KnownFields(true) and the shape walk below
// still cover the root, `play`, `room.room` and every site key, and a typo
// in any of them is still refused at its own path.

// Approved gameplay values (the play block's policies, and the ref type
// monster placements carry).
const (
	transparentVoid     = "transparent"
	brightLighting      = "bright"
	centreCoverStanding = "centre-covered"
	monstersRefType     = "monsters"
)

// Field-error messages. Constant so the same defect always reports the same
// words.
const (
	errRequired      = "is required"
	errNotNull       = "must not be null"
	errNotAString    = "must be a string"
	errNotAnInteger  = "must be an integer"
	errNotABool      = "must be true or false"
	errNotAMapping   = "must be a mapping"
	errNotAList      = "must be a list"
	errLiveSceneProp = "must name a live scene prop"
	errDuplicateID   = "duplicate id"
)

// DecodeSingleRoom strictly reads one lossless single-room dungeon YAML (v3)
// document.
//
// # What it refuses
//
// Strictness is about the file, not just the decoded value. Unknown keys,
// empty input, a second YAML document, duplicate keys and scalar kinds that
// do not fit their field all fail with the offending thing named — an unknown
// key at its PATH and with the keys that shape does take (rpg-project#481, R2;
// unknown_key.go), the rest at the line — over the GAMEPLAY grammar: the root,
// `play`, `room` above the presentation, and `room.room` with every site key. A v3 document is COMPLETE there: every
// required field must be authored, because a key the decoder never saw would
// silently become the Go zero value. Missing play values, monster cells,
// declaration flags and footprint corners are each refused at the YAML path
// that is wrong.
//
// Explicit null is never a v3 value. Optional gameplay fields (partyStart, a
// monster's faction, a creature's orders) may be ABSENT, but an authored
// null is refused: absence means "not supplied", null means "supplied
// nothing", and silently reading both as zero would lose that distinction.
// Integer fields (the version fields and every cell q/r) must be authored as
// integer scalars, because yaml.v3 would otherwise truncate 0.5 into the int
// 0 without a word; string identifiers must be authored as text for the same
// reason.
//
// # What it validates
//
// play carries only its approved values. Walkable cells are unique and stay
// inside the workspace radius; monster and party-start cells are required
// and integral, but whether they STAND is the compiler's question, not a
// floor-membership rule.
//
// propDeclarations must name live scene props; arrangementDeclarations are
// template declarations and may name IDs that were never instantiated.
// Declaration flags and footprints are validated identically for both.
// Monster refs are checked for grammar and the `monsters` type only — no
// rulebook definition is resolved (design C1). `monsterDeclarations` is required and
// may be empty; `partyStart` is optional while editing, so a draft without
// it is valid source, and no playable result is implied by any successful
// decode.
//
// # What it does NOT validate
//
// The presentation (`room.coordinateFrame`, `room.workspace`, `room.scene`).
// It is carried as the authored nodes it is, and only the values play
// depends on are read out of it — single_room_lowering.go names them, and the
// codec that owns the scene is the one that judges the rest
// (rpg-project#479, R3).
//
// The returned spec preserves exactly what was authored: no defaults, no
// repairs, no second scene shape. Every defect comes back as one
// [*ValidationError] listing each [FieldError] at its YAML path.
func DecodeSingleRoom(in SingleRoomDecodeInput) (*SingleRoomDecodeResult, error) {
	if len(bytes.TrimSpace(in.Source)) == 0 {
		return nil, singleRoomErrors("source: empty document")
	}
	dec := yaml.NewDecoder(bytes.NewReader(in.Source))
	dec.KnownFields(true)
	var spec SingleRoomSpec
	if err := dec.Decode(&spec); err != nil {
		// An unknown key comes back at its path and in this package's own
		// words, the same translation [Decode] runs — see unknown_key.go. The
		// two dialects share the shapes and they share the refusal
		// (rpg-project#481, R2).
		return nil, &ValidationError{Errors: decodeErrors(err, in.Source)}
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, singleRoomErrors("source: more than one YAML document")
		}
		return nil, singleRoomErrors(err.Error())
	}
	var root yaml.Node
	if err := yaml.Unmarshal(in.Source, &root); err != nil {
		return nil, singleRoomErrors(err.Error())
	}
	_, errs := validateSingleRoom(&spec)
	errs = append(errs, sourceShapeErrors(&root)...)
	if len(errs) > 0 {
		sortFieldErrors(errs)
		return nil, &ValidationError{Errors: errs}
	}
	return &SingleRoomDecodeResult{Spec: &spec}, nil
}

func singleRoomErrors(msg string) error {
	return &ValidationError{Errors: []FieldError{{Message: msg}}}
}

// sortFieldErrors gives the error list one deterministic order regardless of
// map iteration, so the same file always reports the same list.
func sortFieldErrors(errs []FieldError) {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		return errs[i].Message < errs[j].Message
	})
}

// # Source shape validation
//
// yaml.v3 decodes a missing key into the Go zero value and truncates float
// scalars into Go ints without a word, so presence, null-ness and scalar
// kind are judged on the ORIGINAL nodes. The shape walk knows the v3 schema;
// the decoded-value walk below knows the semantics. Together they leave no
// silent default: every accepted field was authored, and every authored
// field has the type its Go field claims.

// errSink appends one defect at its YAML path.
type errSink func(path, message string)

func documentRoot(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		return root.Content[0]
	}
	return root
}

// childNode returns the value node for key in a mapping, or nil when absent.
// The struct decode honors YAML anchors, aliases and merge keys, so the node
// walk honors them too: an alias resolves to its anchor, and a `<<` merged
// mapping supplies the value only when the key is not authored here.
func childNode(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return resolveNode(m.Content[i+1])
		}
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Tag == mergeKeyTag {
			if found := mergedChild(m.Content[i+1], key, 0); found != nil {
				return found
			}
		}
	}
	return nil
}

const (
	mergeKeyTag = "!!merge"
	// maxNodeHops bounds alias and merge chasing. Struct decode refuses an
	// alias that contains itself, so this cap only keeps the walk from
	// hanging on a file decode should have refused.
	maxNodeHops = 16
)

// resolveNode follows an alias to the node the author anchored.
func resolveNode(n *yaml.Node) *yaml.Node {
	for i := 0; n != nil && n.Kind == yaml.AliasNode && n.Alias != nil && i < maxNodeHops; i++ {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.AliasNode {
		return nil
	}
	return n
}

// mergedChild looks key up inside a `<<` merge source: a mapping or a
// sequence of mappings, each of which may itself merge further maps.
func mergedChild(n *yaml.Node, key string, depth int) *yaml.Node {
	if depth > maxNodeHops {
		return nil
	}
	n = resolveNode(n)
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key && n.Content[i].Tag != mergeKeyTag {
				return resolveNode(n.Content[i+1])
			}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Tag == mergeKeyTag {
				if found := mergedChild(n.Content[i+1], key, depth+1); found != nil {
					return found
				}
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if found := mergedChild(item, key, depth+1); found != nil {
				return found
			}
		}
	}
	return nil
}

func isNull(n *yaml.Node) bool { return n != nil && n.Tag == "!!null" }

// requiredNode reports a missing key, or an authored null under a required
// key, and returns the value node when usable.
func requiredNode(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := childNode(m, key)
	if n == nil {
		add(fieldPath(p, key), errRequired)
		return nil
	}
	if isNull(n) {
		add(fieldPath(p, key), errNotNull)
		return nil
	}
	return n
}

// optionalNode permits absence but refuses an authored null under an
// optional key: leaving heightScale out means normal height, writing null
// means the author deleted a number, and that is not the same event.
func optionalNode(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := childNode(m, key)
	if n == nil {
		return nil
	}
	if isNull(n) {
		add(fieldPath(p, key), errNotNull)
		return nil
	}
	return n
}

func requireString(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := requiredNode(m, key, p, add)
	if n != nil && n.Tag != "!!str" {
		add(fieldPath(p, key), errNotAString)
		return nil
	}
	return n
}

// requireInteger demands an integer SCALAR: `2.0` and `0.5` are !!float
// scalars that yaml.v3 would silently truncate into the Go int 2 or 0.
func requireInteger(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := requiredNode(m, key, p, add)
	if n != nil && n.Tag != "!!int" {
		add(fieldPath(p, key), errNotAnInteger)
		return nil
	}
	return n
}

func requireBoolean(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := requiredNode(m, key, p, add)
	if n != nil && n.Tag != "!!bool" {
		add(fieldPath(p, key), errNotABool)
		return nil
	}
	return n
}

// requireNumber accepts any non-null scalar: yaml.v3 already refuses text
// that is not numeric, and .nan/.inf are refused by the finite check on the
// decoded value.
func requireNumber(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	return requiredNode(m, key, p, add)
}

func requireMapping(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := requiredNode(m, key, p, add)
	if n != nil && n.Kind != yaml.MappingNode {
		add(fieldPath(p, key), errNotAMapping)
		return nil
	}
	return n
}

func requireSequence(m *yaml.Node, key, p string, add errSink) *yaml.Node {
	n := requiredNode(m, key, p, add)
	if n != nil && n.Kind != yaml.SequenceNode {
		add(fieldPath(p, key), errNotAList)
		return nil
	}
	return n
}

func fieldPath(p, key string) string {
	if p == "" {
		return key
	}
	return p + "." + key
}

// sourceShapeErrors reads the whole v3 document node by node and reports
// every required key that is absent, every authored null, and every scalar
// whose kind does not match its field.
func sourceShapeErrors(root *yaml.Node) []FieldError {
	var e []FieldError
	add := func(p, m string) { e = append(e, FieldError{Path: p, Message: m}) }
	doc := resolveNode(documentRoot(root))
	if doc == nil || doc.Kind != yaml.MappingNode {
		add("", "the single room source must be a mapping")
		return e
	}
	requireInteger(doc, "version", "", add)
	requireString(doc, "key", "", add)
	if play := requireMapping(doc, "play", "", add); play != nil {
		requireString(play, "void", "play", add)
		requireString(play, "lighting", "play", add)
		requireString(play, "standing", "play", add)
	}
	// THE SITE SCOPE IS WALKED BEFORE THE ROOM, so a typo in a faction is
	// reported as the typo it is rather than as a room problem — the order
	// the web's own decoder keeps (singleRoomDungeon.ts).
	siteScopeShape(doc, add)
	// THE RECORDS BEFORE THE ROOM, for [siteScopeShape]'s reason one key
	// over: a `holds:` inside the room names one of these, so a typo in a
	// record is reported as the typo it is rather than as a holder problem.
	intelShape(doc, add)
	// AND THE SECRETS, before the room for [intelShape]'s reason one key
	// over: a record's `reveals` names one of these, so a typo in a
	// concealment is reported as the typo it is rather than as a record
	// problem (rpg-project#490).
	concealmentsShape(doc, add)
	// AND THE THREE KEYS THE RUN ITSELF IS MADE OF, still before the room:
	// a scenario binds the things the room places, so a typo in an exit is
	// reported as the typo it is rather than as a binding problem
	// (rpg-project#488, single_room_gameplay.go).
	runShape(doc, add)
	if room := requireMapping(doc, "room", "", add); room != nil {
		roomShape(room, add)
	}
	return e
}

// roomShape walks the room's GAMEPLAY keys. `coordinateFrame`, `workspace`
// and `scene` are deliberately absent from it: the presentation is content
// and this decoder reads only the values play depends on out of it, at
// single_room_lowering.go's own paths (rpg-project#479, R3).
func roomShape(room *yaml.Node, add errSink) {
	requireInteger(room, "version", "room", add)
	requireString(room, "id", "room", add)
	requireString(room, "name", "room", add)
	if gameplay := requireMapping(room, "room", "room", add); gameplay != nil {
		gameplayShape(gameplay, add)
	}
}

// entryShape requires a list entry to be a non-null mapping before its
// fields can be read.
func entryShape(n *yaml.Node, p string, add errSink) {
	n = resolveNode(n)
	if n == nil || isNull(n) {
		add(p, errNotNull)
		return
	}
	if n.Kind != yaml.MappingNode {
		add(p, errNotAMapping)
	}
}

// optionalReference permits absence but refuses an authored null or an
// authored empty string: a reference left out is absent, while "" is a name
// that names nothing.
func optionalReference(m *yaml.Node, key, p string, add errSink) {
	n := optionalNode(m, key, p, add)
	if n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!str" && n.Value == "" {
		add(fieldPath(p, key), "must not be empty")
	}
}

func gameplayShape(gp *yaml.Node, add errSink) {
	requireString(gp, "implicitRegionId", "room.room", add)
	if cells := requireSequence(gp, "walkableHexes", "room.room", add); cells != nil {
		for i, c := range cells.Content {
			cellShape(c, fmt.Sprintf("room.room.walkableHexes[%d]", i), add)
		}
	}
	if decls := requireMapping(gp, "propDeclarations", "room.room", add); decls != nil {
		for i := 0; i+1 < len(decls.Content); i += 2 {
			declarationShape(decls.Content[i+1], "room.room.propDeclarations."+decls.Content[i].Value, add)
		}
	}
	if arrangements := requireMapping(gp, "arrangementDeclarations", "room.room", add); arrangements != nil {
		for i := 0; i+1 < len(arrangements.Content); i += 2 {
			arrangementShape(arrangements.Content[i+1],
				"room.room.arrangementDeclarations."+arrangements.Content[i].Value, add)
		}
	}
	if ps := optionalNode(gp, "partyStart", "room.room", add); ps != nil {
		cellShape(ps, "room.room.partyStart", add)
	}
	if monsters := requireSequence(gp, "monsterDeclarations", "room.room", add); monsters != nil {
		for i, m := range monsters.Content {
			monsterShape(m, fmt.Sprintf("room.room.monsterDeclarations[%d]", i), add)
		}
	}
	monsterBindingsShape(gp, add)
	doorBindingsShape(gp, add)
	propBindingsShape(gp, add)
}

// cellShape reads an axial cell. Both coordinates are required, and the
// original scalars must be integers: yaml.v3 truncates 0.5 into the Go int
// 0, which would quietly move a monster to the origin.
func cellShape(c *yaml.Node, p string, add errSink) {
	c = resolveNode(c)
	if c == nil || isNull(c) {
		add(p, errNotNull)
		return
	}
	if c.Kind != yaml.MappingNode {
		add(p, errNotAMapping)
		return
	}
	requireInteger(c, "q", p, add)
	requireInteger(c, "r", p, add)
}

func monsterShape(m *yaml.Node, p string, add errSink) {
	m = resolveNode(m)
	entryShape(m, p, add)
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	requireString(m, "id", p, add)
	requireString(m, "ref", p, add)
	if sc := requireMapping(m, "startingCell", p, add); sc != nil {
		startingCellShape(sc, p+".startingCell", add)
	}
}

// startingCellShape reads a monster's start: a required location cell beside
// an optional facing. The location is a cell in the author's own frame; the
// facing is a word the value pass validates against the closed set, and this
// walk only pins its shape.
func startingCellShape(sc *yaml.Node, p string, add errSink) {
	if c := requiredNode(sc, "location", p, add); c != nil {
		cellShape(c, p+".location", add)
	}
	// Optional, and refused when authored null: an author deleting the word
	// wrote a different event than one who never wrote it. The word itself
	// is [monsterValues]'s to validate against the closed set.
	if f := optionalNode(sc, "facing", p, add); f != nil && f.Tag != "!!str" {
		add(fieldPath(p, "facing"), errNotAString)
	}
}

// declarationShape is the shared shape for prop and arrangement-template
// declarations: both flags and all four footprint scalars must be authored,
// because false and 0 are real gameplay values, not stand-ins for "never
// said".
func declarationShape(d *yaml.Node, p string, add errSink) {
	d = resolveNode(d)
	if d == nil || isNull(d) {
		add(p, errNotNull)
		return
	}
	if d.Kind != yaml.MappingNode {
		add(p, errNotAMapping)
		return
	}
	requireBoolean(d, "blocksMovement", p, add)
	requireBoolean(d, "blocksLineOfSight", p, add)
	if f := requireMapping(d, "footprint", p, add); f != nil {
		for _, k := range [...]string{"width", "depth", "offsetX", "offsetZ"} {
			requireNumber(f, k, p+".footprint", add)
		}
	}
}

// arrangementShape reads one arrangement-ID -> template-ID -> declaration
// map. Template IDs are NOT required to name live scene props.
func arrangementShape(v *yaml.Node, p string, add errSink) {
	v = resolveNode(v)
	if v == nil || isNull(v) {
		add(p, errNotNull)
		return
	}
	if v.Kind != yaml.MappingNode {
		add(p, errNotAMapping)
		return
	}
	for i := 0; i+1 < len(v.Content); i += 2 {
		declarationShape(v.Content[i+1], p+"."+v.Content[i].Value, add)
	}
}

// # Decoded-value validation
//
// The shape walk guarantees every GAMEPLAY field was authored with the right
// kind. This walk asks whether the authored values are legal: approved
// enums, finite bounded footprints, declaration owners that exist, and the
// handful of presentation values play reads (single_room_lowering.go).

// acceptedRootVersions are the ROOT document versions this decoder speaks,
// lowest first. 4 is the seam, landed AHEAD OF ITS KEYS.
//
// A version is a statement about what a file MAY CONTAIN. Two waves want v4:
// the authored-door contract (rpg-project#468, consumer rpg-dnd5e-web#1117)
// and the site scope plus `monsterBindings` (rpg-project#477, consumer
// rpg-dnd5e-web#1136). The decoder accepts 4 BEFORE either wave's keys exist,
// and the bump lands once so no second bump follows: each key then arrives
// INSIDE a version rather than behind a new one.
//
// Only the ROOT version describes the site document. The embedded room draft
// is its own separately-versioned artifact (the web persists it as
// `rpg-room-authoring-draft` v3), so its version stays 3: root 4 with room 3
// is the only combination the web can currently produce.
//
// 4 accepts exactly the keys 3 does and nothing more, so the version buys no
// leniency. A file containing nothing new has no reason to claim otherwise,
// and v3 therefore behaves exactly as it always did.
var acceptedRootVersions = [...]int{3, 4}

// validateSingleRoom judges one decoded document, and hands back the
// lowering of its presentation beside the defects — one walk, so the values
// the compile carries out are exactly the ones validation looked at.
func validateSingleRoom(s *SingleRoomSpec) (roomRead, []FieldError) {
	var e []FieldError
	add := func(p, m string) { e = append(e, FieldError{Path: p, Message: m}) }
	if !slices.Contains(acceptedRootVersions[:], s.Version) {
		add("version", fmt.Sprintf("unsupported version %d (want 3 or 4)", s.Version))
	}
	if s.Key == "" {
		add("key", errRequired)
	}
	playValues(s.Play, add)
	if s.Room.Version != 3 {
		add("room.version", fmt.Sprintf("unsupported version %d (want 3)", s.Room.Version))
	}
	if s.Room.ID == "" {
		add("room.id", errRequired)
	}
	if s.Room.Name == "" {
		add("room.name", errRequired)
	}
	// THE LOWERING (single_room_lowering.go): the values play reads out of the
	// authored presentation, and every one it needed and could not read,
	// named at its own source path. Nothing else about the scene is judged.
	read, defects := readRoom(&s.Room)
	e = append(e, defects...)
	gameplayValues(&s.Room.Gameplay, read, add)
	// THE SITE SCOPE AND THE ORDERS LAST, and judged by the one gameplay
	// grammar (grammar.go, see single_room_site.go for this dialect's half of
	// the call): every refusal about a faction, a disposition, a membership or
	// an orders block is the one an author already gets from the other
	// dialect, at this dialect's paths.
	siteGrammar(s, add)

	return read, e
}

func playValues(play SingleRoomPlay, add errSink) {
	if play.Void != transparentVoid {
		add("play.void", "must be transparent")
	}
	if play.Lighting != brightLighting {
		add("play.lighting", "must be bright")
	}
	if play.Standing != centreCoverStanding {
		add("play.standing", "must be centre-covered")
	}
}

func gameplayValues(gp *RoomGameplaySource, read roomRead, add errSink) {
	if gp.ImplicitRegionID == "" {
		add("room.room.implicitRegionId", errRequired)
	}
	walkableValues(gp.WalkableHexes, read.WorkspaceHexRadius, read.WorkspaceKnown, add)
	propDeclarationValues(gp.PropDeclarations, read.ItemIDs, add)
	arrangementDeclarationValues(gp.ArrangementDeclarations, add)
	monsterValues(gp.MonsterDeclarations, add)
}

// walkableValues reports duplicate cells and cells outside the painted
// workspace. A monster's own cell is checked for shape only: whether it
// STANDS is decided by final standability at compile time, never by floor
// membership here.
//
// known says whether `room.workspace.hexRadius` was authored as a finite
// number. When it was not, the lowering has already named that at its own
// path and the floor bound is not applied on top of it — one missing number
// reports as one defect, not as every cell being off a floor of radius zero.
func walkableValues(cells []RoomCell, radius float64, known bool, add errSink) {
	seen := make(map[RoomCell]string, len(cells))
	for i, c := range cells {
		p := fmt.Sprintf("room.room.walkableHexes[%d]", i)
		if _, ok := seen[c]; ok {
			add(p, "duplicate cell")
		}
		seen[c] = p
		if known && float64(cubeDistance(c)) > radius {
			add(p, "outside the workspace floor")
		}
	}
}

// cubeDistance is the axial cube metric: max(|q|, |r|, |-q-r|).
//
// SATURATING, NOT WRAPPING. |MinInt| has no positive int, and negating or
// summing an extreme cell would wrap around and measure a cell at the edge
// of the int range as if it sat at the origin — a floor hex accepted because
// its distance came back negative and no radius is smaller than that. Every
// workspace radius is a two-digit number, so a coordinate magnitude beyond a
// tiny bound is outside the floor already and the metric saturates rather
// than lies. Legal cells measure exactly as the unwrapped metric always did.
func cubeDistance(c RoomCell) int {
	const metricBound = 1 << 40
	q, r := saturatingAbs(c.Q), saturatingAbs(c.R)
	if q > metricBound || r > metricBound {
		return math.MaxInt
	}
	return max(q, r, saturatingAbs(-c.Q-c.R))
}

// saturatingAbs is |v| for every int: MinInt saturates to MaxInt instead of
// wrapping back to negative.
func saturatingAbs(v int) int {
	if v == math.MinInt {
		return math.MaxInt
	}
	return abs(v)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func propDeclarationValues(decls map[string]RoomPropDeclaration, itemIDs map[string]bool, add errSink) {
	for id, d := range decls {
		p := "room.room.propDeclarations." + id
		if !itemIDs[id] {
			add(p, errLiveSceneProp)
		}
		footprintValue(d.Footprint, p+".footprint", add)
	}
}

// arrangementDeclarationValues validates template declarations with the same
// footprint rules, without requiring any live owner: template IDs are
// copies, not requirements that unused templates become live actors.
func arrangementDeclarationValues(decls map[string]map[string]RoomPropDeclaration, add errSink) {
	for arr, templates := range decls {
		for tpl, d := range templates {
			footprintValue(d.Footprint,
				"room.room.arrangementDeclarations."+arr+"."+tpl+".footprint", add)
		}
	}
}

// footprintValue bounds a declaration's local rectangle: the editor allows
// sides from 0.1 to 12 scene units and offsets within ±12 of the owner, so
// free negative/fractional poses stay free while nonsense stays refused.
func footprintValue(f RoomFootprint, p string, add errSink) {
	boundFootprint(f.Width, 0.1, 12, p+".width", add)
	boundFootprint(f.Depth, 0.1, 12, p+".depth", add)
	boundFootprint(f.OffsetX, -12, 12, p+".offsetX", add)
	boundFootprint(f.OffsetZ, -12, 12, p+".offsetZ", add)
}

// boundFootprint is footprintValue's one bound check — the decoded-value
// half the scene validator owns for the scene, kept local for the gameplay
// declarations that are not scene content.
func boundFootprint(v, lo, hi float64, p string, add errSink) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		add(p, "must be finite")
		return
	}
	if v < lo || v > hi {
		add(p, fmt.Sprintf("must be between %g and %g", lo, hi))
	}
}

func monsterValues(monsters []RoomMonsterSource, add errSink) {
	seen := make(map[string]bool, len(monsters))
	for i, m := range monsters {
		p := fmt.Sprintf("room.room.monsterDeclarations[%d]", i)
		if m.ID == "" {
			add(p+".id", errRequired)
		}
		if m.ID != "" && seen[m.ID] {
			add(p+".id", errDuplicateID)
		}
		seen[m.ID] = true
		parsed, err := core.ParseString(m.Ref)
		if err != nil {
			add(p+".ref", "invalid ref: "+err.Error())
		} else if parsed.Type != monstersRefType {
			add(p+".ref", "must reference monsters")
		}
		// The facing word, when authored, is one of the eight true-compass
		// names — the same [facings] set the prop path speaks. The value is
		// carried verbatim and never turned into an angle.
		if m.StartingCell.Facing != "" && !facings[m.StartingCell.Facing] {
			add(p+".startingCell.facing", fmt.Sprintf("%q is not a compass direction: a facing is one of n|ne|e|se|s|sw|w|nw", m.StartingCell.Facing))
		}
	}
}
