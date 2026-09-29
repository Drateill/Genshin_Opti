# Artifact Optimizer

A Genshin Impact artifact build optimizer: import a **GOOD** (Genshin Open
Object Description) inventory export, configure a character/target
set/constraints, and get the top-N build combinations ranked by Crit Value
from a real branch-and-bound solver.

This implements the design in `../project/Artifact Optimizer App.dc.html`
(the "Forge" visual direction chosen in `../chats/chat1.md`) against the
architecture in `../uploads/genshin-artifact-optimizer-spec.md` — Go
backend, React frontend, GOOD JSON parsed and solved server-side.

## Structure

- `backend/` — Go API (chi router). Parses GOOD exports, holds the current
  import in memory, and runs the branch-and-bound solver.
- `frontend/` — React + TypeScript (Vite). Recreates the Forge UI from the
  prototype and talks to the backend over HTTP.

## Running locally

**Backend** (listens on `:8080` by default; set `PORT` to override):

```sh
cd backend
go run ./cmd/server
```

**Frontend** (listens on `:5173`, proxies `/api/*` to `:8080` — see
`frontend/vite.config.ts`):

```sh
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173`.

## What's implemented

- **Import** — paste raw GOOD JSON, drop a `.json` file, or load a
  deterministic sample export. Parsing (`backend/internal/good`) is
  tolerant of the per-tool key variations the spec calls out: alternate
  substat field names/casing, a precomputed main-stat value if the export
  includes one (otherwise it's estimated from level/rarity), missing
  level/rarity, null vs. omitted `location`.
- **Configure** — character (from your import, joined against a small
  reference stat table — see below), target set (any set your import
  actually contains), per-slot main-stat filters, minimum-stat constraints,
  top-N.
- **Solve** — `backend/internal/solver` is a real branch-and-bound search:
  fixed slot order, branches over which single slot may be off-set (a
  4-piece bonus only needs 4 of 5 pieces on-set), prunes using a
  best-remaining-slots upper bound, and treats minimum constraints as hard
  accept/reject gates rather than post-hoc annotations. Backed by table
  tests in `backend/internal/solver/solver_test.go`.
- **Results** — ranked list, per-slot build detail, total stats, and
  constraint pass/fail, matching the prototype's Forge visual design
  (dark console, pyro-orange + teal glow, monospace numerics), including
  the accent/solver-stats/default-top-N tweaks the prototype exposed.

## Known scope limits (V1, per the spec's own roadmap)

- **Objective**: Crit Value only. The spec's V2 real-damage objective
  (per-character talent scaling, set 2pc/4pc effects, full DMG formula)
  needs an external character/talent reference DB and is explicitly
  deferred there.
- **Character reference data**: base ATK/HP/CritRate/CritDMG/Energy
  Recharge for the 3 characters the prototype shipped with (Hu Tao, Raiden
  Shogun, Kamisato Ayaka) live in `backend/internal/chardb` as a small
  static table, standing in for the "import from genshin-db or similar"
  step the spec defers to V2. A character in your GOOD import that isn't in
  this table still shows up in the roster (so you can see it was imported)
  but can't be solved for until it's added to `chardb.Chars`.
- **Persistence**: the import is held in memory for the running backend
  process only, per the spec ("GOOD JSON gardé en mémoire au runtime").
  Postgres persistence is called out as optional in the spec and isn't
  implemented.
