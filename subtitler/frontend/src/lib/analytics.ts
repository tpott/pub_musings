/**
 * Analytics tracking client for Subtitler
 *
 * Provides simple event tracking with visitor ID persistence and UTM parameter capture.
 * Privacy-first: minimal PII, local storage only, no third-party services.
 */

const ANALYTICS_API = '/api/analytics/events';
const VISITOR_ID_KEY = 'visitor_id';

/**
 * Generate or retrieve visitor ID from localStorage
 */
function getVisitorId(): string {
  if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
    // Server-side rendering or no localStorage support
    return crypto.randomUUID();
  }

  let visitorId = localStorage.getItem(VISITOR_ID_KEY);
  if (!visitorId) {
    visitorId = crypto.randomUUID();
    localStorage.setItem(VISITOR_ID_KEY, visitorId);
  }
  return visitorId;
}

/**
 * Extract UTM parameters from current URL
 */
function getUTMParams(): {
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
} {
  if (typeof window === 'undefined') {
    return {};
  }

  const params = new URLSearchParams(window.location.search);
  const utmParams: Record<string, string> = {};

  const source = params.get('utm_source');
  const medium = params.get('utm_medium');
  const campaign = params.get('utm_campaign');

  if (source) utmParams.utm_source = source;
  if (medium) utmParams.utm_medium = medium;
  if (campaign) utmParams.utm_campaign = campaign;

  return utmParams;
}

/**
 * Track an analytics event
 *
 * @param eventName - Name of the event (e.g., "signup_completed", "upload_started")
 * @param properties - Optional additional properties for the event
 */
export async function trackEvent(
  eventName: string,
  properties: Record<string, any> = {}
): Promise<void> {
  if (typeof window === 'undefined') {
    // Don't track on server-side rendering
    return;
  }

  const visitorId = getVisitorId();
  const utmParams = getUTMParams();

  try {
    await fetch(ANALYTICS_API, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        visitor_id: visitorId,
        event_name: eventName,
        properties,
        ...utmParams,
      }),
    });
  } catch (error) {
    // Fail silently - don't disrupt user experience
    console.error('Analytics tracking failed:', error);
  }
}

/**
 * Track a page view
 */
export function trackPageView(): void {
  if (typeof window === 'undefined') {
    return;
  }

  trackEvent('page_view', {
    page: window.location.pathname,
    referrer: document.referrer || undefined,
  });
}

/**
 * Track an experiment view (A/B test variant shown to user)
 *
 * @param experimentId - ID of the experiment (e.g., "EXP001")
 * @param variant - Variant shown (e.g., "A", "B", "C")
 */
export function trackExperimentView(
  experimentId: string,
  variant: string
): void {
  trackEvent('experiment_viewed', {
    experiment_id: experimentId,
    variant,
  });
}

/**
 * Track an experiment conversion (user completed experiment goal)
 *
 * @param experimentId - ID of the experiment
 * @param variant - Variant the user saw
 * @param goal - Goal that was completed (e.g., "signup", "upload")
 */
export function trackExperimentConversion(
  experimentId: string,
  variant: string,
  goal: string
): void {
  trackEvent('experiment_converted', {
    experiment_id: experimentId,
    variant,
    goal,
  });
}

/**
 * Get the current visitor ID
 */
export function getCurrentVisitorId(): string {
  return getVisitorId();
}
