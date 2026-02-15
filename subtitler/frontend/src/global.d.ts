/** Global Window interface extensions for cross-component state */
interface Window {
	/** Currently open video ID in the videos modal (set by videos-modal.ts, read by FeedbackButton) */
	currentVideoId: string | null;
	/** Cookie consent API (set by CookieConsent.astro) */
	cookieConsent?: {
		hasConsent: () => boolean;
		hasDeclined: () => boolean;
		hasMadeChoice: () => boolean;
		showBanner: () => void;
	};
}
