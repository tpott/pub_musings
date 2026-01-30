import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setupPreferences, type PreferencesElements } from './settings-preferences';

// --- Mocks ---

vi.mock('./bionic', () => ({
	isBionicEnabled: vi.fn(),
	getBionicFixation: vi.fn(),
	setBionicEnabled: vi.fn(),
	setBionicFixation: vi.fn(),
	toBionicHTML: vi.fn(),
}));

import {
	isBionicEnabled,
	getBionicFixation,
	setBionicEnabled,
	setBionicFixation,
	toBionicHTML,
} from './bionic';

const mockIsBionicEnabled = vi.mocked(isBionicEnabled);
const mockGetBionicFixation = vi.mocked(getBionicFixation);
const mockSetBionicEnabled = vi.mocked(setBionicEnabled);
const mockSetBionicFixation = vi.mocked(setBionicFixation);
const mockToBionicHTML = vi.mocked(toBionicHTML);

function makeMockElements(): PreferencesElements {
	return {
		bionicToggle: {
			checked: false,
			value: '',
			addEventListener: vi.fn(),
		} as unknown as HTMLInputElement,
		bionicOptions: {
			classList: { add: vi.fn(), remove: vi.fn() },
		} as unknown as HTMLElement,
		fixationSlider: {
			value: '40',
			addEventListener: vi.fn(),
		} as unknown as HTMLInputElement,
		fixationValue: {
			textContent: '',
		} as unknown as HTMLElement,
		previewText: {
			innerHTML: '',
			textContent: '',
		} as unknown as HTMLElement,
	};
}

describe('settings-preferences', () => {
	let els: PreferencesElements;

	beforeEach(() => {
		vi.clearAllMocks();
		els = makeMockElements();
		mockIsBionicEnabled.mockReturnValue(false);
		mockGetBionicFixation.mockReturnValue(0.4);
		mockToBionicHTML.mockReturnValue('<strong>The</strong> quick');
	});

	describe('setupPreferences', () => {
		describe('initialization', () => {
			it('should read bionic enabled state from storage', () => {
				setupPreferences(els);
				expect(mockIsBionicEnabled).toHaveBeenCalled();
			});

			it('should read fixation value from storage', () => {
				setupPreferences(els);
				expect(mockGetBionicFixation).toHaveBeenCalled();
			});

			it('should set toggle checked to false when bionic is disabled', () => {
				mockIsBionicEnabled.mockReturnValue(false);
				setupPreferences(els);
				expect(els.bionicToggle.checked).toBe(false);
			});

			it('should set toggle checked to true when bionic is enabled', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				expect(els.bionicToggle.checked).toBe(true);
			});

			it('should set slider value from fixation percentage', () => {
				mockGetBionicFixation.mockReturnValue(0.35);
				setupPreferences(els);
				expect(els.fixationSlider.value).toBe('35');
			});

			it('should display fixation percentage text', () => {
				mockGetBionicFixation.mockReturnValue(0.45);
				setupPreferences(els);
				expect(els.fixationValue.textContent).toBe('45%');
			});

			it('should round fixation percentage for display', () => {
				mockGetBionicFixation.mockReturnValue(0.4);
				setupPreferences(els);
				expect(els.fixationValue.textContent).toBe('40%');
			});
		});

		describe('preview when disabled', () => {
			it('should show plain text preview when bionic is disabled', () => {
				mockIsBionicEnabled.mockReturnValue(false);
				setupPreferences(els);
				expect(els.previewText.textContent).toBe(
					'The quick brown fox jumps over the lazy dog.'
				);
			});

			it('should add disabled-section class when bionic is disabled', () => {
				mockIsBionicEnabled.mockReturnValue(false);
				setupPreferences(els);
				expect(
					(els.bionicOptions.classList as unknown as { add: ReturnType<typeof vi.fn> }).add
				).toHaveBeenCalledWith('disabled-section');
			});
		});

		describe('preview when enabled', () => {
			it('should show bionic HTML preview when bionic is enabled', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				expect(mockToBionicHTML).toHaveBeenCalled();
				expect(els.previewText.innerHTML).toBe('<strong>The</strong> quick');
			});

			it('should pass fixation percent to toBionicHTML', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				mockGetBionicFixation.mockReturnValue(0.45);
				setupPreferences(els);
				expect(mockToBionicHTML).toHaveBeenCalledWith(
					'The quick brown fox jumps over the lazy dog.',
					{ fixationPercent: 0.45 }
				);
			});

			it('should remove disabled-section class when bionic is enabled', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				expect(
					(els.bionicOptions.classList as unknown as { remove: ReturnType<typeof vi.fn> }).remove
				).toHaveBeenCalledWith('disabled-section');
			});
		});

		describe('event listeners', () => {
			it('should register change listener on toggle', () => {
				setupPreferences(els);
				expect(els.bionicToggle.addEventListener).toHaveBeenCalledWith(
					'change',
					expect.any(Function)
				);
			});

			it('should register input listener on slider', () => {
				setupPreferences(els);
				expect(els.fixationSlider.addEventListener).toHaveBeenCalledWith(
					'input',
					expect.any(Function)
				);
			});
		});

		describe('toggle change handler', () => {
			function triggerToggleChange(elements: PreferencesElements) {
				const addEventListenerMock = vi.mocked(elements.bionicToggle.addEventListener);
				const changeHandler = addEventListenerMock.mock.calls.find(
					(call) => call[0] === 'change'
				)?.[1] as () => void;
				changeHandler();
			}

			it('should call setBionicEnabled with true when toggled on', () => {
				setupPreferences(els);
				els.bionicToggle.checked = true;
				triggerToggleChange(els);
				expect(mockSetBionicEnabled).toHaveBeenCalledWith(true);
			});

			it('should call setBionicEnabled with false when toggled off', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				els.bionicToggle.checked = false;
				triggerToggleChange(els);
				expect(mockSetBionicEnabled).toHaveBeenCalledWith(false);
			});

			it('should update preview when toggled on', () => {
				setupPreferences(els);
				els.bionicToggle.checked = true;
				triggerToggleChange(els);
				expect(mockToBionicHTML).toHaveBeenCalled();
				expect(els.previewText.innerHTML).toBe('<strong>The</strong> quick');
			});

			it('should show plain text preview when toggled off', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				els.bionicToggle.checked = false;
				triggerToggleChange(els);
				expect(els.previewText.textContent).toBe(
					'The quick brown fox jumps over the lazy dog.'
				);
			});

			it('should add disabled-section when toggled off', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				vi.mocked((els.bionicOptions.classList as unknown as { add: ReturnType<typeof vi.fn> }).add).mockClear();
				els.bionicToggle.checked = false;
				triggerToggleChange(els);
				expect(
					(els.bionicOptions.classList as unknown as { add: ReturnType<typeof vi.fn> }).add
				).toHaveBeenCalledWith('disabled-section');
			});

			it('should remove disabled-section when toggled on', () => {
				setupPreferences(els);
				vi.mocked((els.bionicOptions.classList as unknown as { remove: ReturnType<typeof vi.fn> }).remove).mockClear();
				els.bionicToggle.checked = true;
				triggerToggleChange(els);
				expect(
					(els.bionicOptions.classList as unknown as { remove: ReturnType<typeof vi.fn> }).remove
				).toHaveBeenCalledWith('disabled-section');
			});
		});

		describe('slider input handler', () => {
			function triggerSliderInput(elements: PreferencesElements) {
				const addEventListenerMock = vi.mocked(elements.fixationSlider.addEventListener);
				const inputHandler = addEventListenerMock.mock.calls.find(
					(call) => call[0] === 'input'
				)?.[1] as () => void;
				inputHandler();
			}

			it('should update fixation value text', () => {
				setupPreferences(els);
				els.fixationSlider.value = '35';
				triggerSliderInput(els);
				expect(els.fixationValue.textContent).toBe('35%');
			});

			it('should call setBionicFixation with decimal value', () => {
				setupPreferences(els);
				els.fixationSlider.value = '45';
				triggerSliderInput(els);
				expect(mockSetBionicFixation).toHaveBeenCalledWith(0.45);
			});

			it('should update preview with new fixation value when enabled', () => {
				mockIsBionicEnabled.mockReturnValue(true);
				setupPreferences(els);
				mockToBionicHTML.mockClear();
				els.bionicToggle.checked = true;
				els.fixationSlider.value = '50';
				triggerSliderInput(els);
				expect(mockToBionicHTML).toHaveBeenCalledWith(
					'The quick brown fox jumps over the lazy dog.',
					{ fixationPercent: 0.5 }
				);
			});

			it('should show plain text when slider changes but toggle is off', () => {
				setupPreferences(els);
				els.bionicToggle.checked = false;
				els.fixationSlider.value = '50';
				triggerSliderInput(els);
				expect(els.previewText.textContent).toBe(
					'The quick brown fox jumps over the lazy dog.'
				);
			});
		});
	});
});
