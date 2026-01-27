/**
 * Utility for tracking and estimating transcription processing time
 *
 * Stores historical processing speeds (seconds per second of video) in localStorage
 * to estimate time remaining for future transcriptions.
 */

const STORAGE_KEY = 'subtitler:processing_speed_history';
const MAX_HISTORY_SIZE = 10;

interface ProcessingRecord {
  videoDuration: number;     // Video duration in seconds
  processingTime: number;    // Total processing time in seconds
  timestamp: number;         // When this was recorded
}

interface SpeedHistory {
  records: ProcessingRecord[];
}

/**
 * Get the processing speed history from localStorage
 */
function getHistory(): SpeedHistory {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const history = JSON.parse(stored) as SpeedHistory;
      if (history && Array.isArray(history.records)) {
        return history;
      }
    }
  } catch (e) {
    console.error('Failed to parse processing speed history:', e);
  }
  return { records: [] };
}

/**
 * Save a processing record to history
 */
export function recordProcessingTime(videoDuration: number, processingTime: number): void {
  if (videoDuration <= 0 || processingTime <= 0) {
    return; // Invalid data
  }

  const history = getHistory();

  // Add new record
  history.records.push({
    videoDuration,
    processingTime,
    timestamp: Date.now()
  });

  // Keep only the most recent records
  if (history.records.length > MAX_HISTORY_SIZE) {
    history.records = history.records.slice(-MAX_HISTORY_SIZE);
  }

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(history));
  } catch (e) {
    console.error('Failed to save processing speed history:', e);
  }
}

/**
 * Get the average processing speed ratio (processing_seconds / video_seconds)
 * Returns null if no history available
 */
export function getAverageSpeedRatio(): number | null {
  const history = getHistory();

  if (history.records.length === 0) {
    return null;
  }

  // Calculate weighted average (more recent records weighted higher)
  let totalWeight = 0;
  let weightedSum = 0;

  history.records.forEach((record, index) => {
    const ratio = record.processingTime / record.videoDuration;
    const weight = index + 1; // Later records get higher weight
    weightedSum += ratio * weight;
    totalWeight += weight;
  });

  return weightedSum / totalWeight;
}

/**
 * Estimate total processing time for a given video duration
 * Returns null if no historical data available
 */
export function estimateTotalTime(videoDuration: number): number | null {
  const avgRatio = getAverageSpeedRatio();
  if (avgRatio === null) {
    return null;
  }
  return videoDuration * avgRatio;
}

/**
 * Estimate time remaining given video duration, progress percentage, and elapsed time
 *
 * This uses a combination of:
 * - Historical average (when available)
 * - Current session's actual progress rate
 *
 * @param videoDuration Video duration in seconds
 * @param progressPercent Current progress (0-100)
 * @param elapsedSeconds Time elapsed since transcription started
 * @returns Estimated seconds remaining, or null if cannot estimate
 */
export function estimateTimeRemaining(
  videoDuration: number,
  progressPercent: number,
  elapsedSeconds: number
): number | null {
  // Need at least some progress to estimate based on current rate
  if (progressPercent <= 0) {
    // Use historical average if available
    const estimated = estimateTotalTime(videoDuration);
    return estimated;
  }

  if (progressPercent >= 100) {
    return 0;
  }

  // Calculate based on current progress rate
  const progressFraction = progressPercent / 100;
  const estimatedTotal = elapsedSeconds / progressFraction;
  const remaining = estimatedTotal - elapsedSeconds;

  // If we have historical data, blend it with current rate
  // This helps smooth out early estimates
  const historicalTotal = estimateTotalTime(videoDuration);
  if (historicalTotal !== null && progressPercent < 50) {
    // Weight historical more when progress is low
    const historicalWeight = 1 - (progressPercent / 50);
    const currentWeight = progressPercent / 50;
    const blendedTotal = (historicalTotal * historicalWeight) + (estimatedTotal * currentWeight);
    const blendedRemaining = blendedTotal - elapsedSeconds;
    return Math.max(0, blendedRemaining);
  }

  return Math.max(0, remaining);
}

/**
 * Format a duration in seconds to a human-readable string
 */
export function formatTimeRemaining(seconds: number | null): string {
  if (seconds === null) {
    return 'Estimating...';
  }

  seconds = Math.ceil(seconds);

  if (seconds < 0) {
    return 'Almost done...';
  }

  if (seconds < 60) {
    return seconds === 1 ? '~1 second remaining' : `~${seconds} seconds remaining`;
  }

  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;

  if (minutes < 60) {
    if (remainingSeconds === 0) {
      return minutes === 1 ? '~1 minute remaining' : `~${minutes} minutes remaining`;
    }
    return `~${minutes}m ${remainingSeconds}s remaining`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `~${hours}h ${remainingMinutes}m remaining`;
}

/**
 * Check if we have enough historical data to make estimates
 */
export function hasHistoricalData(): boolean {
  const history = getHistory();
  return history.records.length > 0;
}

/**
 * Clear processing speed history (for testing or user request)
 */
export function clearHistory(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch (e) {
    console.error('Failed to clear processing speed history:', e);
  }
}
