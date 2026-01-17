import type { CliArgs } from './types.js';

// CLI argument parsing
export function parseArgs(): CliArgs {
  const args = process.argv.slice(2);

  // Parse flags
  let list = false;
  let all = false;
  let first: number | undefined;
  let select: number[] | undefined;
  let dryRun = false;
  let followIframe: boolean | undefined;
  const positional: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--list') {
      list = true;
    } else if (arg === '--all') {
      all = true;
    } else if (arg === '--first') {
      const nextArg = args[i + 1];
      if (nextArg !== undefined && !nextArg.startsWith('-')) {
        first = parseInt(nextArg, 10);
        i++; // Skip the number
      }
    } else if (arg === '--select') {
      const nextArg = args[i + 1];
      if (nextArg !== undefined && !nextArg.startsWith('-')) {
        select = nextArg.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n));
        i++; // Skip the value
      }
    } else if (arg === '--dry-run') {
      dryRun = true;
    } else if (arg === '--follow-iframe') {
      followIframe = true;
    } else if (arg === '--no-follow-iframe') {
      followIframe = false;
    } else if (!arg.startsWith('-')) {
      positional.push(arg);
    }
  }

  if (positional.length === 0) {
    console.log('Usage: bun run job-crawler.ts <careers-url> [pattern] [flags]');
    console.log('');
    console.log('Patterns: greenhouse, lever, workday, ashby, generic (default)');
    console.log('');
    console.log('Flags:');
    console.log('  --list              List jobs only (no crawling) - useful for testing extraction');
    console.log('  --all               Select all jobs and crawl (non-interactive)');
    console.log('  --first N           Select first N jobs and crawl');
    console.log('  --select "N,M,..."  Select specific job indices to crawl (comma-separated)');
    console.log('  --dry-run           Show what would be crawled without actually crawling');
    console.log('  --follow-iframe     Force following job board iframes');
    console.log('  --no-follow-iframe  Stay on parent page, don\'t follow iframes');
    console.log('');
    console.log('Examples:');
    console.log('  bun run job-crawler.ts "https://example.com/careers#open-positions"');
    console.log('  bun run job-crawler.ts "https://example.com/careers" lever');
    console.log('  bun run job-crawler.ts "https://example.com/careers" --list');
    console.log('  bun run job-crawler.ts "https://example.com/careers" --first 3');
    console.log('  bun run job-crawler.ts "https://example.com/careers" --select "1,3,5"');
    console.log('  bun run job-crawler.ts "https://example.com/careers" --all --dry-run');
    process.exit(1);
  }

  return {
    careersUrl: positional[0],
    pattern: positional[1],
    list,
    all,
    first,
    select,
    dryRun,
    followIframe,
  };
}
