/**
 * Audio conversion pipeline: WAV -> WebM/Opus -> base64 chunks.
 */
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { synthesizeWAV } from './piper-client';
import type { SynthesizedAudio } from './types';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const DEFAULT_CHUNK_SIZE = 4096; // 4KB, same as real-services.spec.ts

/** Convert WAV audio buffer to WebM/Opus using ffmpeg. */
export function wavToWebmOpus(wav: Buffer): Buffer {
  // ffmpeg reads from stdin, writes to stdout
  // -f wav: input format
  // -c:a libopus: Opus codec
  // -f webm: output container
  return execFileSync('ffmpeg', [
    '-f', 'wav',
    '-i', 'pipe:0',
    '-c:a', 'libopus',
    '-b:a', '32k',
    '-f', 'webm',
    'pipe:1',
  ], {
    input: wav,
    maxBuffer: 10 * 1024 * 1024, // 10MB
    stdio: ['pipe', 'pipe', 'pipe'],
  });
}

/** Split a buffer into base64-encoded chunks. */
export function splitIntoChunks(
  data: Buffer,
  chunkSize: number = DEFAULT_CHUNK_SIZE,
): string[] {
  const chunks: string[] = [];
  for (let i = 0; i < data.length; i += chunkSize) {
    chunks.push(data.subarray(i, i + chunkSize).toString('base64'));
  }
  return chunks;
}

/** Synthesize text via Piper, convert to WebM/Opus, and split into chunks. */
export async function synthesizeAndChunk(
  text: string,
  piperUrl: string,
): Promise<SynthesizedAudio> {
  const wav = await synthesizeWAV(text, piperUrl);
  const webm = wavToWebmOpus(wav);
  const chunks = splitIntoChunks(webm);
  return { text, chunks };
}

/** Read a .webm fixture file and split into base64 chunks. */
export function fixtureToChunks(fixturePath: string): string[] {
  const resolvedPath = path.isAbsolute(fixturePath)
    ? fixturePath
    : path.resolve(__dirname, '..', '..', '..', '..', 'tests', 'fixtures', fixturePath);
  const data = fs.readFileSync(resolvedPath);
  return splitIntoChunks(data);
}
