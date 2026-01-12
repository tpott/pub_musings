// Type definitions for job-crawler

export interface CliArgs {
  careersUrl: string;
  pattern?: string;
  // New non-interactive flags
  list?: boolean;      // --list: List jobs only (no crawling)
  all?: boolean;       // --all: Select all and crawl
  first?: number;      // --first N: Select first N jobs
  dryRun?: boolean;    // --dry-run: Show what would be crawled
}

export interface JobListing {
  index: number;
  title: string;
  url: string;
  department?: string;
  location?: string;
}

export interface JobBoardPattern {
  name: string;
  urlPattern: RegExp;
  selectors: string[];
  contentSelector: string;
}

export interface HarEntry {
  startedDateTime: string;
  time: number;
  request: {
    method: string;
    url: string;
    headers: Array<{ name: string; value: string }>;
  };
  response: {
    status: number;
    statusText: string;
    headers: Array<{ name: string; value: string }>;
    content: {
      size: number;
      mimeType: string;
      text?: string;
    };
  };
}

export interface HarLog {
  version: string;
  creator: { name: string; version: string };
  entries: HarEntry[];
}
