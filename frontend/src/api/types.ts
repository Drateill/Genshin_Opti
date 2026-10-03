export interface Stat {
  key: string;
  value: number;
}

export interface Artifact {
  id: number;
  setKey: string;
  slotKey: 'flower' | 'plume' | 'sands' | 'goblet' | 'circlet';
  level: number;
  rarity: number;
  mainStatKey: string;
  mainStatValue: number;
  location: string;
  lock: boolean;
  substats: Stat[];
}

export interface TalentLevels {
  auto: number;
  skill: number;
  burst: number;
}

export interface GoodExport {
  format: string;
  version: number;
  source?: string;
  characters: { key: string; level: number; ascension: number; constellation: number; talent?: TalentLevels }[];
  artifacts: Artifact[];
}

export interface ImportSummary {
  characters: number;
  artifacts: number;
  sets: number;
  setKeys: string[];
}

export interface RosterEntry {
  key: string;
  name: string;
  element: string;
  level: number;
  constellation: number;
  recSet: string;
  dmgKey: string;
  weaponType: string;
  rarity?: number; // 4 or 5, absent when unknown
  talent: TalentLevels;
  icon?: string;
  known: boolean;
  equippedWeapon?: string;
  equippedWeaponKnown: boolean;
}

export interface WeaponOption {
  id: number;
  key: string;
  name: string;
  type: string;
  rarity: number;
  level: number;
  refinement: number;
  count: number;
  atk: number;
  subStatKey?: string;
  subStatValue?: number;
  icon?: string;
  known: boolean;
}

export interface SetInfo {
  key: string;
  name: string;
  short: string;
  description?: string;
  icon?: string;
  counts: Record<string, number>;
}

export interface CharacterInsight {
  key: string;
  name: string;
  element?: string; // stable English key (Pyro, Hydro, ...), absent when unknown
  elementLabel?: string; // localized display text
  weaponType?: string;
  rarity?: number;
  level: number;
  constellation: number;
  avgTalent: number;
  known: boolean;
  weaponName?: string;
  weaponRarity?: number;
  weaponRefinement?: number;
  weaponLevel?: number;
  artifactsEquipped: number;
  avgArtifactLevel: number;
  investment: number;
  breakdown: ScoreBreakdown;
}

export interface ScoreComponent {
  fraction: number; // 0-1, how "complete" this component is
  weight: number; // this component's share of the 100-point total, e.g. 20
  points: number; // fraction * weight
}

export interface ScoreBreakdown {
  level: ScoreComponent;
  constellation: ScoreComponent;
  talents: ScoreComponent;
  weapon: ScoreComponent;
  artifactCount: ScoreComponent;
  artifactQuality: ScoreComponent;
}

export interface ElementCount {
  element: string;
  label: string;
  count: number;
}

export interface WeaponTypeStat {
  type: string;
  equipped: number;
  benched: number;
}

export interface SetCount {
  key: string;
  name: string;
  short: string;
  count: number;
}

export interface IdleWeaponGroup {
  key: string;
  name: string;
  type: string;
  rarity: number;
  count: number;
  maxLevel: number;
  maxRefinement: number;
}

export interface InsightsOverview {
  characters: number;
  artifacts: number;
  weapons: number;
  fiveStarArtifacts: number;
  lockedArtifacts: number;
  maxLevelArtifacts: number;
  equippedArtifacts: number;
  benchedArtifacts: number;
  equippedWeapons: number;
  benchedWeapons: number;
  avgInvestment: number;
}

export interface ArtifactQuality {
  id: number;
  setKey: string;
  setName: string;
  setShort: string;
  slotKey: string;
  level: number;
  mainStatKey: string;
  location?: string;
  locationName?: string;
  lock: boolean;
  critValue: number;
  rollQuality?: number; // "RV%", absent when not computable (non-5-star)
}

export interface RollQualityBucket {
  label: string;
  count: number;
}

export interface ArtifactQualityOverview {
  ratedArtifacts: number;
  avgCritValue: number;
  avgRollQuality: number;
  belowAverageCount: number;
}

export interface InsightsResponse {
  overview: InsightsOverview;
  characters: CharacterInsight[];
  elements: ElementCount[];
  weaponTypes: WeaponTypeStat[];
  sets: SetCount[];
  idleWeapons: IdleWeaponGroup[];
  artifactQuality: ArtifactQualityOverview;
  rollQualityBuckets: RollQualityBucket[];
  hiddenGems: ArtifactQuality[];
  fodderCandidates: ArtifactQuality[];
}

export interface StatRange {
  min?: number;
  max?: number;
}

export interface SolveRequest {
  characterKey: string;
  targetSetKey: string;
  targetSetKey2?: string; // non-empty switches the solver to dual 2pc+2pc mode
  weaponId?: number;
  slotConstraints: Record<string, string[]>;
  constraints: Record<string, StatRange>;
  objective: string;
  topN: number;
  includeEquippedByOthers: boolean;
  lang?: string;
  teamElements?: string[]; // 0-3 teammate element keys ("Pyro", "Hydro", ...) for elemental resonance
}

export interface BuildTotals {
  critRate: number;
  critDMG: number;
  elementalMastery: number;
  energyRecharge: number;
  atk: number;
  hp: number;
  elementalDMG: number;
}

export interface ConstraintCheck {
  key: string;
  min?: number;
  max?: number;
  value: number;
  met: boolean;
}

export interface BuildResult {
  rank: number;
  critValue: number;
  pieces: Artifact[];
  complete: boolean;
  onSetCount: number;
  onSetCount2?: number; // meaningful only when SolveRequest.targetSetKey2 was set
  totals: BuildTotals;
  checks: ConstraintCheck[];
  allMet: boolean;
}

export interface SolveResponse {
  builds: BuildResult[];
  currentBuild?: BuildResult;
  meta: string;
  reason?: string;
  solveMs: number;
  prunedBranches: number;
  timedOut?: boolean;
}

export interface SolveProgress {
  tested: number;
  total: number; // an upper bound only — tested may never exactly reach it, even once done
  done: boolean;
  result?: SolveResponse;
  error?: string;
}

export interface ApiErrorBody {
  error: string;
}

// --- Damage calculator ---
// Talent %ATK multipliers are auto-extracted from genshin-db (see
// tools/gendata/generate.js) for every character's normal-attack/skill/
// burst; character-passive and artifact-set 2pc/4pc buffs are NOT included
// (genshin-db has no numeric data for those) — approximate them with
// ExtraBonus instead. See backend/internal/damage for the full formula.

export interface TalentMultiplier {
  label: string;
  values: number[]; // %ATK per talent level 1-15, index 0 = level 1
}

export interface CharacterTalents {
  auto: TalentMultiplier[];
  skill: TalentMultiplier[];
  burst: TalentMultiplier[];
}

export type TalentGroup = 'auto' | 'skill' | 'burst';

export interface EnemyPreset {
  key: string;
  name: string;
  level: number;
  resPct: number;
  category: 'common' | 'boss';
}

export interface ExtraBonus {
  dmgPct: number;
  critRatePct: number;
  critDmgPct: number;
  flatATK: number;
  atkPct: number;
  defShredPct: number;
  resShredPct: number;
}

export interface EnemyInput {
  level: number;
  resPct: number;
}

export type ReactionType =
  | ''
  | 'vaporize'
  | 'melt'
  | 'overloaded'
  | 'superconduct'
  | 'electrocharged'
  | 'swirl'
  | 'shattered'
  | 'burning'
  | 'bloom'
  | 'burgeon'
  | 'hyperbloom'
  | 'spread'
  | 'aggravate';

export interface ReactionInput {
  type: ReactionType;
  bonusPct: number;
}

export interface DamageRequest {
  characterKey: string;
  casterLevel: number;
  totals: BuildTotals;
  talentGroup: TalentGroup;
  talentLevel: number;
  componentIndices?: number[]; // empty/omitted = every component in the group
  extra: ExtraBonus;
  enemy: EnemyInput;
  reaction: ReactionInput;
}

export interface ComponentResult {
  label: string;
  nonCrit: number;
  crit: number;
  average: number;
}

export interface DamageResponse {
  components: ComponentResult[];
  total: number;
  reactionDamage?: number;
}
