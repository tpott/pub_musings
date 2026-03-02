/**
 * Piper TTS HTTP client.
 * Same protocol as backend/tts/piper.go Synthesize method.
 */

/**
 * Synthesize text to WAV audio via Piper TTS server.
 * POST JSON { text, length_scale } and receive raw WAV bytes.
 */
export async function synthesizeWAV(
  text: string,
  piperUrl: string,
): Promise<Buffer> {
  const body = JSON.stringify({ text, length_scale: 1.0 });
  const resp = await fetch(piperUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  });
  if (!resp.ok) {
    const errText = await resp.text().catch(() => 'unknown error');
    throw new Error(`Piper synthesis failed (${resp.status}): ${errText}`);
  }
  const arrayBuf = await resp.arrayBuffer();
  return Buffer.from(arrayBuf);
}
