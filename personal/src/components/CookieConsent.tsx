import { useState, useEffect } from 'react';

const CONSENT_KEY = 'cookie-consent';

type ConsentValue = 'accepted' | 'rejected' | null;

export default function CookieConsent() {
  const [consent, setConsent] = useState<ConsentValue>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const storedConsent = localStorage.getItem(CONSENT_KEY) as ConsentValue;
    if (storedConsent === 'accepted' || storedConsent === 'rejected') {
      setConsent(storedConsent);
      if (storedConsent === 'accepted') {
        loadAnalytics();
      }
    } else {
      // Show banner after a short delay for better UX
      const timer = setTimeout(() => setIsVisible(true), 1000);
      return () => clearTimeout(timer);
    }
  }, []);

  const loadAnalytics = () => {
    // Dispatch event that Analytics.astro listens for
    window.dispatchEvent(new CustomEvent('cookie-consent-accepted'));
  };

  const handleAccept = () => {
    localStorage.setItem(CONSENT_KEY, 'accepted');
    setConsent('accepted');
    setIsVisible(false);
    loadAnalytics();
  };

  const handleReject = () => {
    localStorage.setItem(CONSENT_KEY, 'rejected');
    setConsent('rejected');
    setIsVisible(false);
  };

  // Don't render anything if consent has been given or not visible yet
  if (consent !== null || !isVisible) {
    return null;
  }

  return (
    <div className="cookie-banner" role="dialog" aria-labelledby="cookie-title" aria-describedby="cookie-desc">
      <div className="cookie-content">
        <h3 id="cookie-title">Cookie Settings</h3>
        <p id="cookie-desc">
          This website uses cookies for analytics to understand how visitors interact with the site.
          No personal data is sold or shared with third parties.
        </p>
        <div className="cookie-actions">
          <button onClick={handleReject} className="reject">
            Decline
          </button>
          <button onClick={handleAccept} className="accept">
            Accept
          </button>
        </div>
      </div>

      <style>{`
        .cookie-banner {
          position: fixed;
          bottom: 0;
          left: 0;
          right: 0;
          background: var(--color-bg, #fafafa);
          border-top: 1px solid var(--color-border, #e5e5e5);
          padding: 1rem;
          box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.1);
          z-index: 1000;
          animation: slideUp 0.3s ease-out;
        }

        @keyframes slideUp {
          from {
            transform: translateY(100%);
          }
          to {
            transform: translateY(0);
          }
        }

        .cookie-content {
          max-width: 720px;
          margin: 0 auto;
        }

        h3 {
          font-size: 1rem;
          margin-bottom: 0.5rem;
        }

        p {
          font-size: 0.875rem;
          color: var(--color-text-muted, #666);
          margin-bottom: 1rem;
          line-height: 1.5;
        }

        .cookie-actions {
          display: flex;
          gap: 0.75rem;
          justify-content: flex-end;
        }

        button {
          padding: 0.5rem 1rem;
          font-size: 0.875rem;
          font-weight: 500;
          border-radius: 6px;
          cursor: pointer;
          transition: all 0.2s;
        }

        .reject {
          background: transparent;
          border: 1px solid var(--color-border, #e5e5e5);
          color: var(--color-text-muted, #666);
        }

        .reject:hover {
          background: var(--color-border, #e5e5e5);
        }

        .accept {
          background: var(--color-primary, #2563eb);
          border: none;
          color: white;
        }

        .accept:hover {
          background: var(--color-primary-hover, #1d4ed8);
        }

        @media (max-width: 480px) {
          .cookie-actions {
            flex-direction: column;
          }

          button {
            width: 100%;
          }
        }
      `}</style>
    </div>
  );
}
