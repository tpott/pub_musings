/** Configuration for the PBT runner. */
export interface PBTConfig {
  /** Piper TTS server URL (e.g., http://localhost:5000). */
  piperUrl: string;
  /** Default assertion timeout in milliseconds. */
  timeout: number;
}

/** Audio synthesized by Piper and converted to WebM/Opus chunks. */
export interface SynthesizedAudio {
  /** Original text that was synthesized. */
  text: string;
  /** Base64-encoded 4KB chunks of WebM/Opus data. */
  chunks: string[];
}
