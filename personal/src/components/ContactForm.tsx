import { useState, type FormEvent } from 'react';
import { validateEmail, validateRequired, validateMinLength } from '../utils/validation';

interface FormData {
  name: string;
  email: string;
  message: string;
  website: string; // honeypot field
}

interface FormErrors {
  name?: string;
  email?: string;
  message?: string;
}

type SubmitStatus = 'idle' | 'submitting' | 'success' | 'error';

export default function ContactForm() {
  const [formData, setFormData] = useState<FormData>({
    name: '',
    email: '',
    message: '',
    website: '', // honeypot - should remain empty
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [status, setStatus] = useState<SubmitStatus>('idle');
  const [errorMessage, setErrorMessage] = useState('');

  const validate = (): boolean => {
    const newErrors: FormErrors = {};

    if (!validateRequired(formData.name)) {
      newErrors.name = 'Name is required';
    }

    if (!validateRequired(formData.email)) {
      newErrors.email = 'Email is required';
    } else if (!validateEmail(formData.email)) {
      newErrors.email = 'Please enter a valid email address';
    }

    if (!validateRequired(formData.message)) {
      newErrors.message = 'Message is required';
    } else if (!validateMinLength(formData.message, 10)) {
      newErrors.message = 'Message must be at least 10 characters';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    setStatus('submitting');
    setErrorMessage('');

    try {
      const apiUrl = import.meta.env.PUBLIC_API_URL;
      const response = await fetch(`${apiUrl}/api/contact`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: formData.name,
          email: formData.email,
          message: formData.message,
          website: formData.website, // honeypot
        }),
      });

      if (response.ok) {
        setStatus('success');
        setFormData({ name: '', email: '', message: '', website: '' });
      } else if (response.status === 429) {
        setStatus('error');
        setErrorMessage('Too many requests. Please try again later.');
      } else {
        const data = await response.json().catch(() => ({}));
        setStatus('error');
        setErrorMessage(data.error || 'Something went wrong. Please try again.');
      }
    } catch {
      setStatus('error');
      setErrorMessage('Failed to send message. Please try again later.');
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
    // Clear error when user starts typing
    if (errors[name as keyof FormErrors]) {
      setErrors((prev) => ({ ...prev, [name]: undefined }));
    }
  };

  if (status === 'success') {
    return (
      <div className="success-message" data-hydrated="true">
        <h2>Message Sent!</h2>
        <p>Thank you for reaching out. I'll get back to you as soon as possible.</p>
        <button type="button" onClick={() => setStatus('idle')}>
          Send another message
        </button>
      </div>
    );
  }

  // noValidate disables browser-native validation so our custom React validation runs instead
  return (
    <form onSubmit={handleSubmit} className="contact-form" data-hydrated="true" noValidate>
      <div className="form-group">
        <label htmlFor="name">Name</label>
        <input
          type="text"
          id="name"
          name="name"
          value={formData.name}
          onChange={handleChange}
          aria-invalid={!!errors.name}
          aria-describedby={errors.name ? 'name-error' : undefined}
        />
        {errors.name && (
          <span id="name-error" className="error" role="alert">
            {errors.name}
          </span>
        )}
      </div>

      <div className="form-group">
        <label htmlFor="email">Email</label>
        <input
          type="email"
          id="email"
          name="email"
          value={formData.email}
          onChange={handleChange}
          aria-invalid={!!errors.email}
          aria-describedby={errors.email ? 'email-error' : undefined}
        />
        {errors.email && (
          <span id="email-error" className="error" role="alert">
            {errors.email}
          </span>
        )}
      </div>

      <div className="form-group">
        <label htmlFor="message">Message</label>
        <textarea
          id="message"
          name="message"
          rows={5}
          value={formData.message}
          onChange={handleChange}
          aria-invalid={!!errors.message}
          aria-describedby={errors.message ? 'message-error' : undefined}
        />
        {errors.message && (
          <span id="message-error" className="error" role="alert">
            {errors.message}
          </span>
        )}
      </div>

      {/* Honeypot field - hidden from users, bots will fill it */}
      <div className="form-group" style={{ position: 'absolute', left: '-9999px' }} aria-hidden="true">
        <label htmlFor="website">Website</label>
        <input
          type="text"
          id="website"
          name="website"
          value={formData.website}
          onChange={handleChange}
          tabIndex={-1}
          autoComplete="off"
        />
      </div>

      {status === 'error' && (
        <div className="error-banner" role="alert">
          {errorMessage}
        </div>
      )}

      <button type="submit" disabled={status === 'submitting'}>
        {status === 'submitting' ? 'Sending...' : 'Send Message'}
      </button>

      <style>{`
        .contact-form {
          max-width: 500px;
        }

        .form-group {
          margin-bottom: 1.5rem;
        }

        label {
          display: block;
          margin-bottom: 0.5rem;
          font-weight: 500;
        }

        input, textarea {
          width: 100%;
          padding: 0.75rem;
          border: 1px solid var(--color-border, #e5e5e5);
          border-radius: 6px;
          font-size: 1rem;
          font-family: inherit;
          background: var(--color-bg, #fafafa);
          color: var(--color-text, #1a1a1a);
        }

        input:focus, textarea:focus {
          outline: none;
          border-color: var(--color-primary, #2563eb);
          box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
        }

        input[aria-invalid="true"], textarea[aria-invalid="true"] {
          border-color: #dc2626;
        }

        .error {
          display: block;
          color: #dc2626;
          font-size: 0.875rem;
          margin-top: 0.25rem;
        }

        .error-banner {
          background: #fef2f2;
          border: 1px solid #fecaca;
          color: #dc2626;
          padding: 0.75rem 1rem;
          border-radius: 6px;
          margin-bottom: 1rem;
        }

        button {
          background: var(--color-primary, #2563eb);
          color: white;
          border: none;
          padding: 0.75rem 1.5rem;
          font-size: 1rem;
          font-weight: 500;
          border-radius: 6px;
          cursor: pointer;
          transition: background 0.2s;
        }

        button:hover:not(:disabled) {
          background: var(--color-primary-hover, #1d4ed8);
        }

        button:disabled {
          opacity: 0.6;
          cursor: not-allowed;
        }

        .success-message {
          text-align: center;
          padding: 2rem;
          border: 1px solid var(--color-border, #e5e5e5);
          border-radius: 8px;
        }

        .success-message h2 {
          color: #16a34a;
          margin-bottom: 0.5rem;
        }

        .success-message p {
          color: var(--color-text-muted, #666);
          margin-bottom: 1.5rem;
        }

        @media (prefers-color-scheme: dark) {
          .error-banner {
            background: #450a0a;
            border-color: #7f1d1d;
          }
        }
      `}</style>
    </form>
  );
}
