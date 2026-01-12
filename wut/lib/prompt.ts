import type { Page } from 'puppeteer';
import { createInterface } from 'readline';
import type { JobBoardPattern, JobListing } from './types.js';
import { extractJobListings } from './extraction.js';

// Display a page of job listings
export function displayJobPage(listings: JobListing[], page: number, pageSize: number): void {
  const totalPages = Math.ceil(listings.length / pageSize);
  const startIdx = page * pageSize;
  const endIdx = Math.min(startIdx + pageSize, listings.length);
  const pageJobs = listings.slice(startIdx, endIdx);

  console.log(`\n--- Jobs (Page ${page + 1}/${totalPages}) ---`);
  for (const job of pageJobs) {
    console.log(`${job.index}. ${job.title}`);
    if (job.department) console.log(`   Department: ${job.department}`);
    if (job.location) console.log(`   Location: ${job.location}`);
  }
  console.log('');
}

// Prompt user for job selection with pagination
export async function promptForSelection(
  listings: JobListing[],
  page: Page,
  pattern: JobBoardPattern
): Promise<number[]> {
  const pageSize = 20;
  const totalPages = Math.ceil(listings.length / pageSize);
  let currentPage = 0;
  const selectedIndices: number[] = [];

  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  const askQuestion = (prompt: string): Promise<string> => {
    return new Promise((resolve) => {
      rl.question(prompt, resolve);
    });
  };

  while (true) {
    displayJobPage(listings, currentPage, pageSize);

    if (selectedIndices.length > 0) {
      console.log(`Selected so far: ${selectedIndices.join(', ')}`);
    }

    const prompt = totalPages > 1
      ? 'Enter indices (e.g., "1,3,5"), "n"=next, "p"=prev, "r"=refresh, "a"=all, "d"=done, "q"=quit: '
      : 'Enter indices (e.g., "1,3,5"), "r"=refresh, "a"=all, "d"=done, "q"=quit: ';

    const answer = (await askQuestion(prompt)).toLowerCase().trim();

    if (answer === 'q' || answer === 'quit') {
      rl.close();
      return [];
    }

    if (answer === 'd' || answer === 'done') {
      rl.close();
      return selectedIndices;
    }

    if (answer === 'n' || answer === 'next') {
      if (currentPage < totalPages - 1) {
        currentPage++;
      } else {
        console.log('Already on last page.');
      }
      continue;
    }

    if (answer === 'p' || answer === 'prev') {
      if (currentPage > 0) {
        currentPage--;
      } else {
        console.log('Already on first page.');
      }
      continue;
    }

    if (answer === 'r' || answer === 'refresh') {
      console.log('\nRe-extracting job listings from current page state...');
      const newListings = await extractJobListings(page, pattern);
      if (newListings.length === 0) {
        console.log('No jobs found. Try scrolling or adjusting filters.');
        continue;
      }
      // Reset state with new listings
      listings.length = 0;
      listings.push(...newListings);
      selectedIndices.length = 0;
      currentPage = 0;
      console.log(`Found ${listings.length} job listing(s).`);
      continue;
    }

    if (answer === 'a' || answer === 'all') {
      rl.close();
      return listings.map((job) => job.index);
    }

    // Parse indices
    const indices = answer
      .split(',')
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n) && n >= 1 && n <= listings.length);

    if (indices.length === 0) continue;

    for (const idx of indices) {
      if (!selectedIndices.includes(idx)) {
        selectedIndices.push(idx);
      }
    }
    console.log(`Added: ${indices.join(', ')}`);

    // If single page, return immediately after selection
    if (totalPages === 1) {
      rl.close();
      return selectedIndices;
    }

    // Advance to next page after selection
    if (currentPage < totalPages - 1) {
      currentPage++;
    }
  }
}
