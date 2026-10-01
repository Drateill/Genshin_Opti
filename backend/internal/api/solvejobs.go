package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"

	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/model"
	"artifact-optimizer/internal/solver"
)

// A solve can take a long time on a large inventory (branch-and-bound over
// every 5-piece combination), so POST /api/solve just starts the search in
// a goroutine and returns a job ID; the caller polls GET
// /api/solve/{id}/progress for a live tested/total count until it reports
// done, at which point the result (or error) is attached and the job is
// dropped from the store.
type solveJob struct {
	progress solver.Progress
	done     atomic.Bool
	result   model.SolveResponse
	err      string
}

var (
	jobsMu     sync.Mutex
	jobs       = map[string]*solveJob{}
	jobCounter atomic.Int64
)

func newSolveJob() (string, *solveJob) {
	id := fmt.Sprintf("solve-%d", jobCounter.Add(1))
	job := &solveJob{}
	jobsMu.Lock()
	jobs[id] = job
	jobsMu.Unlock()
	return id, job
}

func getSolveJob(id string) (*solveJob, bool) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	return j, ok
}

func deleteSolveJob(id string) {
	jobsMu.Lock()
	delete(jobs, id)
	jobsMu.Unlock()
}

type solveStartResponse struct {
	JobID string `json:"jobId"`
}

type solveProgressResponse struct {
	Tested int64                `json:"tested"`
	Total  int64                `json:"total"` // an upper bound, not a promise Tested will reach it — see solver.Progress
	Done   bool                 `json:"done"`
	Result *model.SolveResponse `json:"result,omitempty"`
	Error  string               `json:"error,omitempty"`
}

func (a *API) handleSolveStart(w http.ResponseWriter, r *http.Request) {
	var req model.SolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid solve request: "+err.Error())
		return
	}
	if req.CharacterKey == "" || req.TargetSetKey == "" {
		writeError(w, http.StatusBadRequest, "characterKey and targetSetKey are required")
		return
	}
	if req.TargetSetKey2 != "" && req.TargetSetKey2 == req.TargetSetKey {
		writeError(w, http.StatusBadRequest, "targetSetKey2 must differ from targetSetKey")
		return
	}
	export := a.store.Get()
	if len(export.Artifacts) == 0 {
		writeError(w, http.StatusConflict, "no inventory imported yet")
		return
	}

	var weaponATK, weaponSubVal float64
	var weaponSubKey string
	if req.WeaponID != nil {
		for _, wp := range export.Weapons {
			if wp.ID == *req.WeaponID {
				weaponATK, weaponSubKey, weaponSubVal, _ = chardb.WeaponStatAt(wp.Key, wp.Level)
				break
			}
		}
	}

	// currentPieces is whatever the character actually has equipped right
	// now (one artifact per slot, in chardb.SlotOrder), so the response can
	// carry a real "current build" alongside the search results for a
	// direct before/after comparison. currentReq scores it with the
	// character's actually-equipped weapon, not the (possibly different)
	// weapon chosen in the request — CurrentBuild is meant to be a true
	// snapshot of what the player has right now.
	equippedBySlot := map[string]model.Artifact{}
	for _, art := range export.Artifacts {
		if art.Location == req.CharacterKey {
			equippedBySlot[art.SlotKey] = art
		}
	}
	var currentPieces []model.Artifact
	for _, slot := range chardb.SlotOrder {
		if art, ok := equippedBySlot[slot]; ok {
			currentPieces = append(currentPieces, art)
		}
	}
	var curWeaponATK, curWeaponSubVal float64
	var curWeaponSubKey string
	for _, wp := range export.Weapons {
		if wp.Location == req.CharacterKey {
			curWeaponATK, curWeaponSubKey, curWeaponSubVal, _ = chardb.WeaponStatAt(wp.Key, wp.Level)
			break
		}
	}
	currentReq := solver.Request{
		CharacterKey: req.CharacterKey, TargetSetKey: req.TargetSetKey, TargetSetKey2: req.TargetSetKey2,
		WeaponATK: curWeaponATK, WeaponSubKey: curWeaponSubKey, WeaponSubValue: curWeaponSubVal,
		Constraints: req.Constraints, TeamElements: req.TeamElements,
	}

	id, job := newSolveJob()
	solveReq := solver.Request{
		CharacterKey: req.CharacterKey, TargetSetKey: req.TargetSetKey, TargetSetKey2: req.TargetSetKey2,
		WeaponATK: weaponATK, WeaponSubKey: weaponSubKey, WeaponSubValue: weaponSubVal,
		SlotConstraints: req.SlotConstraints, Constraints: req.Constraints, TopN: req.TopN,
		IncludeEquippedByOthers: req.IncludeEquippedByOthers,
		Lang:                    req.Lang,
		TeamElements:            req.TeamElements,
		Progress:                &job.progress,
	}
	go func() {
		s := solver.New(export.Artifacts)
		res, err := s.Solve(solveReq)
		if err != nil {
			job.err = err.Error()
			job.done.Store(true)
			return
		}
		if len(currentPieces) > 0 {
			if cur, cerr := s.Evaluate(currentPieces, currentReq); cerr == nil {
				res.CurrentBuild = &cur
			}
		}
		job.result = res
		job.done.Store(true)
	}()

	writeJSON(w, http.StatusOK, solveStartResponse{JobID: id})
}

func (a *API) handleSolveProgress(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := getSolveJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown or already-collected solve job")
		return
	}
	resp := solveProgressResponse{
		Tested: job.progress.Tested.Load(),
		Total:  job.progress.Total.Load(),
		Done:   job.done.Load(),
	}
	if resp.Done {
		if job.err != "" {
			resp.Error = job.err
		} else {
			resp.Result = &job.result
		}
		deleteSolveJob(id)
	}
	writeJSON(w, http.StatusOK, resp)
}
