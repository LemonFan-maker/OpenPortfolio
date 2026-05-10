export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface ProjectCard {
  name: string;
  description: string;
  language: string;
  languages: LanguageStat[];
  stars: number;
  forks: number;
  topics: string[];
  url: string;
  homepage: string;
  updatedAt: string;
}

export interface LanguageStat {
  name: string;
  bytes: number;
  percent: number;
}

export interface LanguageStats {
  totalBytes: number;
  languages: LanguageStat[];
}

export interface ContributionDay {
  date: string;
  count: number;
  color: string;
}

export interface ContributionWeek {
  firstDay: string;
  days: ContributionDay[];
}

export interface ContributionMonth {
  name: string;
  firstDay: string;
  totalWeeks: number;
  year: number;
}

export interface CommitActivity {
  totalContributions: number;
  maxContributions: number;
  weeks: ContributionWeek[];
  months: ContributionMonth[];
}

export interface CommitActivityDetail {
  date: string;
  totalCommits: number;
  commits: CommitRecord[];
  truncated: boolean;
}

export interface CommitRecord {
  sha: string;
  message: string;
  repository: string;
  url: string;
  committedAt: string;
  additions: number;
  deletions: number;
  files: CommitFileDiff[];
  filesChanged: number;
}

export interface CommitFileDiff {
  filename: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
}

export interface RepoDetail extends ProjectCard {
  readme: string;
  languages: LanguageStat[];
  openIssues: number;
}

export interface RepoTreeEntry {
  path: string;
  type: "file" | "dir";
  size: number;
  sha: string;
}

export interface RepoTree {
  defaultBranch: string;
  truncated: boolean;
  entries: RepoTreeEntry[];
}

export interface ProjectSummary {
  shortSummary: string;
  longSummary: string;
  highlights: string[];
  techTags: string[];
}

export interface SiteSummarySection {
  title: string;
  body?: string;
  items?: string[];
}

export interface SiteSummary {
  title: string;
  subtitle: string;
  shortSummary: string;
  longSummary: string;
  sections: SiteSummarySection[];
  techTags: string[];
}

export interface SiteSummaryStatus {
  source: string;
  error?: string;
  cached: boolean;
  summary: SiteSummary;
  generatedAt: string;
}

export interface ProjectSummaryStatus {
  repoName: string;
  source: string;
  error?: string;
  cached: boolean;
  summary: ProjectSummary;
  generatedAt: string;
}

export interface ProjectSummaryTurnLog {
  turn: number;
  prompt: string;
  response: string;
  action: string;
  path?: string;
  query?: string;
  reason?: string;
  toolResult?: string;
  error?: string;
}

export interface ProjectSummaryDebug {
  repoName: string;
  source: string;
  error?: string;
  cached: boolean;
  summary: ProjectSummary;
  detail: RepoDetail;
  tree: RepoTree;
  generatedAt: string;
  turns: ProjectSummaryTurnLog[];
}
