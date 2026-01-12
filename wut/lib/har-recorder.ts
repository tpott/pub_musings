import type { Page, CDPSession } from 'puppeteer';
import type { HarEntry, HarLog } from './types.js';

export interface HarRecorder {
  start: () => Promise<void>;
  stop: () => Promise<HarLog>;
}

// HAR recorder using CDP
export async function createHarRecorder(page: Page): Promise<HarRecorder> {
  const client: CDPSession = await page.createCDPSession();
  const entries: HarEntry[] = [];
  const requestMap = new Map<string, { startTime: number; request: HarEntry['request'] }>();

  return {
    start: async () => {
      await client.send('Network.enable');

      client.on('Network.requestWillBeSent', (params) => {
        const { requestId, request, timestamp } = params;
        requestMap.set(requestId, {
          startTime: timestamp,
          request: {
            method: request.method,
            url: request.url,
            headers: Object.entries(request.headers).map(([name, value]) => ({
              name,
              value: String(value),
            })),
          },
        });
      });

      client.on('Network.responseReceived', (params) => {
        const { requestId, response, timestamp } = params;
        const requestData = requestMap.get(requestId);
        if (requestData === undefined) return;

        const entry: HarEntry = {
          startedDateTime: new Date().toISOString(),
          time: (timestamp - requestData.startTime) * 1000,
          request: requestData.request,
          response: {
            status: response.status,
            statusText: response.statusText,
            headers: Object.entries(response.headers).map(([name, value]) => ({
              name,
              value: String(value),
            })),
            content: {
              size: response.encodedDataLength || 0,
              mimeType: response.mimeType,
            },
          },
        };
        entries.push(entry);
      });
    },
    stop: async () => {
      await client.send('Network.disable');
      client.removeAllListeners();
      return {
        version: '1.2',
        creator: { name: 'job-crawler', version: '1.0.0' },
        entries,
      };
    },
  };
}
