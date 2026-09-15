package world

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type seqCase struct {
	Name         string `json:"name"`
	Spawn        int    `json:"spawn"`
	Sleep        []int  `json:"sleep"`
	Wake         []int  `json:"wake"`
	Dispose      []int  `json:"dispose"`
	WantAlive    int    `json:"want_alive"`
	WantActive   int    `json:"want_active"`
	WantSleeping int    `json:"want_sleeping"`
}

type hierCase struct {
	Name         string `json:"name"`
	Chain        int    `json:"chain"`
	Sleep        []int  `json:"sleep"`
	DisposeRoot  int    `json:"dispose_root"`
	WantAlive    int    `json:"want_alive"`
	WantActive   int    `json:"want_active"`
	WantSleeping int    `json:"want_sleeping"`
}

type lifeFile struct {
	Sequences []seqCase `json:"sequences"`
	Hierarchy hierCase  `json:"hierarchy"`
	Perf      struct {
		Rounds int `json:"rounds"`
	} `json:"perf"`
	Longrun struct {
		Levels int `json:"levels"`
		Batch  int `json:"batch"`
	} `json:"longrun"`
	Golden struct {
		Spawn        int    `json:"spawn"`
		Sleep        []int  `json:"sleep"`
		Dispose      []int  `json:"dispose"`
		WantAlive    int    `json:"want_alive"`
		WantActive   int    `json:"want_active"`
		WantSleeping int    `json:"want_sleeping"`
		WantSpawned  uint64 `json:"want_spawned"`
	} `json:"golden"`
}

func loadLifecycleCases(t *testing.T) lifeFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "lifecycle_cases.json"))
	if err != nil {
		t.Fatalf("read lifecycle_cases.json: %v", err)
	}
	var f lifeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode lifecycle_cases.json: %v", err)
	}
	if len(f.Sequences) == 0 || f.Perf.Rounds <= 0 || f.Longrun.Levels <= 0 {
		t.Fatal("lifecycle_cases.json has no cases")
	}
	return f
}

func mustLifeSpawn(t *testing.T, s *Scene, parent ID) ID {
	t.Helper()
	id, err := s.Spawn(parent)
	if err != nil {
		t.Fatalf("Spawn(%d): %v", parent, err)
	}
	if id == NoEntity || !s.Alive(id) || !s.IsActive(id) {
		t.Fatalf("Spawn(%d) = %d, want a live active id", parent, id)
	}
	return id
}

func lifeAt(ids []ID, i int) ID { return ids[i] }

func applyLifeOp(t *testing.T, s *Scene, ids []ID, op string, idx []int) {
	t.Helper()
	for _, i := range idx {
		if i < 0 || i >= len(ids) {
			t.Fatalf("%s index %d out of %d", op, i, len(ids))
		}
		id := lifeAt(ids, i)
		var err error
		switch op {
		case "sleep":
			err = s.Sleep(id)
		case "wake":
			err = s.Wake(id)
		case "dispose":
			err = s.Dispose(id)
		default:
			t.Fatalf("unknown op %q", op)
		}
		if err != nil {
			t.Fatalf("%s(%d): %v", op, id, err)
		}
	}
}

func checkLifeCounts(t *testing.T, tag string, s *Scene, alive, active, sleeping int) {
	t.Helper()
	if s.Count() != alive {
		t.Errorf("%s count = %d, want %d", tag, s.Count(), alive)
	}
	if s.ActiveCount() != active {
		t.Errorf("%s active = %d, want %d", tag, s.ActiveCount(), active)
	}
	if s.SleepingCount() != sleeping {
		t.Errorf("%s sleeping = %d, want %d", tag, s.SleepingCount(), sleeping)
	}
	if s.ActiveCount()+s.SleepingCount() != s.Count() {
		t.Errorf("%s active+sleeping != count", tag)
	}
}

func expectLifeCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:生灭休眠对:出生即激活,睡停留活,醒回激活,销毁清资源.
func TestLifecycleSequencesFromCases(t *testing.T) {
	f := loadLifecycleCases(t)
	for _, sq := range f.Sequences {
		s := NewScene()
		ids := make([]ID, 0, sq.Spawn)
		for i := 0; i < sq.Spawn; i++ {
			ids = append(ids, mustLifeSpawn(t, &s, NoEntity))
		}
		// Sleeping keeps placement and comps; only Dispose clears them.
		for _, id := range ids {
			loc := Transform{Pos: core.V2(float64(id), 1), Rot: 0, Scale: core.V2(1, 1)}
			if err := s.World().SetTransform(id, loc); err != nil {
				t.Fatalf("%s SetTransform(%d): %v", sq.Name, id, err)
			}
			if err := s.World().AddComp(id, Comp{Kind: "sprite", Ref: "tex/hero"}); err != nil {
				t.Fatalf("%s AddComp(%d): %v", sq.Name, id, err)
			}
		}
		applyLifeOp(t, &s, ids, "sleep", sq.Sleep)
		applyLifeOp(t, &s, ids, "wake", sq.Wake)
		applyLifeOp(t, &s, ids, "dispose", sq.Dispose)
		checkLifeCounts(t, sq.Name, &s, sq.WantAlive, sq.WantActive, sq.WantSleeping)
		disposed := map[int]bool{}
		for _, i := range sq.Dispose {
			disposed[i] = true
		}
		for i, id := range ids {
			if disposed[i] {
				if s.Alive(id) || s.IsActive(id) || s.IsSleeping(id) {
					t.Errorf("%s id[%d] alive after Dispose", sq.Name, i)
				}
				expectLifeCode(t, sq.Name+" state of dead", func() error {
					_, err := s.State(id)
					return err
				}(), core.CodeNotFound)
				if _, err := s.World().Comps(id); core.CodeOf(err) != core.CodeNotFound {
					t.Errorf("%s comps of dead code = %v, want not-found", sq.Name, core.CodeOf(err))
				}
				continue
			}
			st, err := s.State(id)
			if err != nil {
				t.Errorf("%s State(%d): %v", sq.Name, id, err)
				continue
			}
			if (st == LifeSleeping) != s.IsSleeping(id) || (st == LifeActive) != s.IsActive(id) {
				t.Errorf("%s id[%d] flag mismatch", sq.Name, i)
			}
			// Survivors keep their comps.
			got, err := s.World().Comps(id)
			if err != nil || len(got) != 1 || got[0].Kind != "sprite" {
				t.Errorf("%s survivor id[%d] comps = %v,%v, want one sprite", sq.Name, i, got, err)
			}
		}
	}
	// Hierarchy: disposing the root orphans the chain keeping own flags.
	h := f.Hierarchy
	s := NewScene()
	chain := make([]ID, 0, h.Chain)
	for i := 0; i < h.Chain; i++ {
		parent := NoEntity
		if len(chain) > 0 {
			parent = chain[len(chain)-1]
		}
		chain = append(chain, mustLifeSpawn(t, &s, parent))
	}
	applyLifeOp(t, &s, chain, "sleep", h.Sleep)
	if err := s.Dispose(chain[h.DisposeRoot]); err != nil {
		t.Fatalf("%s dispose root: %v", h.Name, err)
	}
	checkLifeCounts(t, h.Name, &s, h.WantAlive, h.WantActive, h.WantSleeping)
	if p, err := s.World().Parent(chain[1]); err != nil || p != NoEntity {
		t.Errorf("%s orphaned parent = %d,%v, want root", h.Name, p, err)
	}
}

// B:重复销毁不崩:二次错码分清,坏链坏号拒收,nil接收器不炸.
func TestLifecycleEdgesNoCrash(t *testing.T) {
	s := NewScene()
	a := mustLifeSpawn(t, &s, NoEntity)
	if err := s.Sleep(a); err != nil {
		t.Fatalf("Sleep: %v", err)
	}
	if err := s.Sleep(a); err != nil {
		t.Errorf("re-sleep: %v, want idempotent nil", err)
	}
	if !s.IsSleeping(a) || s.IsActive(a) {
		t.Error("sleeping flags wrong")
	}
	if err := s.Wake(a); err != nil {
		t.Fatalf("Wake: %v", err)
	}
	if err := s.Wake(a); err != nil {
		t.Errorf("re-wake: %v, want idempotent nil", err)
	}
	if err := s.Dispose(a); err != nil {
		t.Fatalf("Dispose: %v", err)
	}
	expectLifeCode(t, "dispose twice", s.Dispose(a), core.CodeNotFound)
	expectLifeCode(t, "sleep dead", s.Sleep(a), core.CodeNotFound)
	expectLifeCode(t, "wake dead", s.Wake(a), core.CodeNotFound)
	expectLifeCode(t, "sleep missing", s.Sleep(9999), core.CodeNotFound)
	expectLifeCode(t, "wake missing", s.Wake(9999), core.CodeNotFound)
	expectLifeCode(t, "dispose missing", s.Dispose(9999), core.CodeNotFound)
	if _, err := s.Spawn(9999); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("spawn under dead code = %v, want not-found", core.CodeOf(err))
	}
	if s.Alive(a) || s.IsActive(a) || s.IsSleeping(a) {
		t.Error("dead id still reports alive")
	}
	if LifeActive.String() != "active" || LifeSleeping.String() != "sleeping" {
		t.Error("LifeState names moved")
	}

	// Nil scene never panics: writers refuse, getters park.
	var ns *Scene
	if _, err := ns.Spawn(NoEntity); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Spawn code = %v, want invalid-arg", core.CodeOf(err))
	}
	expectLifeCode(t, "nil Sleep", ns.Sleep(1), core.CodeInvalidArg)
	expectLifeCode(t, "nil Wake", ns.Wake(1), core.CodeInvalidArg)
	expectLifeCode(t, "nil Dispose", ns.Dispose(1), core.CodeInvalidArg)
	expectLifeCode(t, "nil State", func() error { _, err := ns.State(1); return err }(), core.CodeNotFound)
	if ns.Alive(1) || ns.IsActive(1) || ns.IsSleeping(1) {
		t.Error("nil liveness true, want parked false")
	}
	if ns.Count() != 0 || ns.ActiveCount() != 0 || ns.SleepingCount() != 0 || ns.Spawned() != 0 {
		t.Error("nil counters nonzero, want parked zero")
	}
	if ns.World() != nil {
		t.Error("nil World non-nil, want nil")
	}
	ns.Clear()
}

// C不适用(纯算数不画画):同脚本双建逐位一致,发号只涨不回头.
func TestLifecycleReplayIdentical(t *testing.T) {
	f := loadLifecycleCases(t)
	build := func() *Scene {
		s := NewScene()
		var ids []ID
		for i := 0; i < f.Golden.Spawn; i++ {
			ids = append(ids, mustLifeSpawn(t, &s, NoEntity))
		}
		applyLifeOp(t, &s, ids, "sleep", f.Golden.Sleep)
		applyLifeOp(t, &s, ids, "dispose", f.Golden.Dispose)
		return &s
	}
	a, b := build(), build()
	if a.Count() != b.Count() || a.ActiveCount() != b.ActiveCount() ||
		a.SleepingCount() != b.SleepingCount() || a.Spawned() != b.Spawned() {
		t.Fatalf("replay diverged: %+v vs %+v", a, b)
	}
	for id := ID(1); id <= ID(f.Golden.Spawn); id++ {
		_, errA := a.State(id)
		_, errB := b.State(id)
		if core.CodeOf(errA) != core.CodeOf(errB) {
			t.Fatalf("replay id %d codes %v vs %v", id, errA, errB)
		}
		if errA == nil {
			sa, _ := a.State(id)
			sb, _ := b.State(id)
			if sa != sb {
				t.Fatalf("replay id %d states %v vs %v", id, sa, sb)
			}
			ma, erA := a.World().WorldMatrix(id)
			mb, erB := b.World().WorldMatrix(id)
			if erA != nil || erB != nil || ma != mb {
				t.Fatalf("replay id %d matrices diverge", id)
			}
		}
	}
}

// D:万次生灭跑得动,耗时有数.
func TestLifecyclePerfTenThousand(t *testing.T) {
	f := loadLifecycleCases(t)
	rounds := f.Perf.Rounds
	s := NewScene()
	// Synthetic churn only (no golden): golden stays in the json file.
	start := time.Now()
	for i := 0; i < rounds; i++ {
		id, err := s.Spawn(NoEntity)
		if err != nil {
			t.Fatalf("round %d Spawn: %v", i, err)
		}
		if err := s.Dispose(id); err != nil {
			t.Fatalf("round %d Dispose: %v", i, err)
		}
	}
	el := time.Since(start)
	t.Logf("lifecycle-perf: %d spawn+dispose rounds in %v (%.1f ns/op)", rounds, el, float64(el.Nanoseconds())/float64(rounds))
	if s.Count() != 0 {
		t.Errorf("after churn count = %d, want 0", s.Count())
	}
	if s.Spawned() != uint64(rounds) {
		t.Errorf("spawned = %d, want ledger %d", s.Spawned(), rounds)
	}
	if math.IsNaN(float64(el.Nanoseconds())) || el.Nanoseconds() <= 0 {
		t.Error("perf clock invalid, benchmark meaningless")
	}
}

// E:跨关不漏:常驻逐位一致,暂存回基线,发号只涨,整关Clear归零不回退.
func TestLifecycleLevelsNoLeak(t *testing.T) {
	f := loadLifecycleCases(t)
	levels, batch := f.Longrun.Levels, f.Longrun.Batch
	s := NewScene()
	keep := mustLifeSpawn(t, &s, NoEntity)
	doze := mustLifeSpawn(t, &s, NoEntity)
	if err := s.Sleep(doze); err != nil {
		t.Fatalf("park resident: %v", err)
	}
	rl := Transform{Pos: core.V2(3, 4), Rot: 0.5, Scale: core.V2(2, 2)}
	if err := s.World().SetTransform(keep, rl); err != nil {
		t.Fatalf("resident transform: %v", err)
	}
	if err := s.World().AddComp(keep, Comp{Kind: "script"}); err != nil {
		t.Fatalf("resident comp: %v", err)
	}
	snapLocal, _ := s.World().Local(keep)
	snapWorld, _ := s.World().WorldOf(keep)
	snapMat, _ := s.World().WorldMatrix(keep)
	snapComps, _ := s.World().Comps(keep)
	snapState, _ := s.State(doze)
	for lv := 0; lv < levels; lv++ {
		born := make([]ID, 0, batch)
		for j := 0; j < batch; j++ {
			id, err := s.Spawn(NoEntity)
			if err != nil {
				t.Fatalf("level %d spawn: %v", lv, err)
			}
			born = append(born, id)
		}
		for _, id := range born {
			if err := s.Dispose(id); err != nil {
				t.Fatalf("level %d dispose(%d): %v", lv, id, err)
			}
			if s.Alive(id) {
				t.Fatalf("level %d: %d alive after dispose", lv, id)
			}
		}
		if s.Count() != 2 {
			t.Fatalf("level %d count = %d, want 2 residents", lv, s.Count())
		}
		if got, _ := s.World().Local(keep); got != snapLocal {
			t.Fatalf("level %d resident moved", lv)
		}
		if got, _ := s.World().WorldOf(keep); got != snapWorld {
			t.Fatalf("level %d resident world moved", lv)
		}
		if got, _ := s.World().WorldMatrix(keep); got != snapMat {
			t.Fatalf("level %d resident matrix moved", lv)
		}
		if got, _ := s.World().Comps(keep); len(got) != len(snapComps) || got[0] != snapComps[0] {
			t.Fatalf("level %d resident comps moved", lv)
		}
		if got, _ := s.State(doze); got != snapState {
			t.Fatalf("level %d parked flag moved", lv)
		}
	}
	if s.Spawned() != uint64(2+levels*batch) {
		t.Errorf("spawned = %d, want ledger %d", s.Spawned(), 2+levels*batch)
	}
	// Level switch drops the living but never rewinds issuance.
	top := s.Spawned()
	s.Clear()
	if s.Count() != 0 || s.ActiveCount() != 0 || s.SleepingCount() != 0 {
		t.Errorf("after Clear count = %d/%d/%d, want 0/0/0", s.Count(), s.ActiveCount(), s.SleepingCount())
	}
	fresh, err := s.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn after Clear: %v", err)
	}
	if uint64(fresh) <= top {
		t.Errorf("post-Clear id %d rewinds issuance (spawned %d)", fresh, top)
	}
}

// F:离屏金对照窗(W4窗免,纯算数):冻结数加形状断言.
func TestLifecycleOffscreenGolden(t *testing.T) {
	f := loadLifecycleCases(t)
	g := f.Golden
	s := NewScene()
	ids := make([]ID, 0, g.Spawn)
	for i := 0; i < g.Spawn; i++ {
		ids = append(ids, mustLifeSpawn(t, &s, NoEntity))
	}
	applyLifeOp(t, &s, ids, "sleep", g.Sleep)
	applyLifeOp(t, &s, ids, "dispose", g.Dispose)
	checkLifeCounts(t, "golden", &s, g.WantAlive, g.WantActive, g.WantSleeping)
	if s.Spawned() != g.WantSpawned {
		t.Errorf("golden spawned = %d, want %d", s.Spawned(), g.WantSpawned)
	}
	// Shape: survivors hold their flags, the disposed hold nothing.
	sleeping := map[int]bool{}
	for _, i := range g.Sleep {
		sleeping[i] = true
	}
	disposed := map[int]bool{}
	for _, i := range g.Dispose {
		disposed[i] = true
	}
	for i, id := range ids {
		switch {
		case disposed[i]:
			if s.Alive(id) {
				t.Errorf("golden id[%d] alive after dispose", i)
			}
		case sleeping[i]:
			if !s.IsSleeping(id) || s.IsActive(id) {
				t.Errorf("golden id[%d] not parked", i)
			}
		default:
			if !s.IsActive(id) || s.IsSleeping(id) {
				t.Errorf("golden id[%d] not running", i)
			}
		}
	}
}
