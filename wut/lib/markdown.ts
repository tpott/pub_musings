import TurndownService from 'turndown';

// HTML to markdown converter
export function createMarkdownConverter(): TurndownService {
  const turndownService = new TurndownService({
    headingStyle: 'atx',
    codeBlockStyle: 'fenced',
  });

  // Remove script and style tags
  turndownService.remove(['script', 'style', 'nav', 'footer', 'header']);

  return turndownService;
}
