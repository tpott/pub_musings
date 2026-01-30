import {
	isBionicEnabled,
	getBionicFixation,
	setBionicEnabled,
	setBionicFixation,
	toBionicHTML,
} from './bionic';

export interface PreferencesElements {
	bionicToggle: HTMLInputElement;
	bionicOptions: HTMLElement;
	fixationSlider: HTMLInputElement;
	fixationValue: HTMLElement;
	previewText: HTMLElement;
}

const PREVIEW_SAMPLE = 'The quick brown fox jumps over the lazy dog.';

export function setupPreferences(els: PreferencesElements) {
	function updateBionicPreview() {
		const enabled = els.bionicToggle.checked;
		const fixation = parseInt(els.fixationSlider.value, 10) / 100;

		if (enabled) {
			els.previewText.innerHTML = toBionicHTML(PREVIEW_SAMPLE, { fixationPercent: fixation });
		} else {
			els.previewText.textContent = PREVIEW_SAMPLE;
		}

		if (enabled) {
			els.bionicOptions.classList.remove('disabled-section');
		} else {
			els.bionicOptions.classList.add('disabled-section');
		}
	}

	function initBionicSettings() {
		const enabled = isBionicEnabled();
		const fixation = getBionicFixation();

		els.bionicToggle.checked = enabled;
		els.fixationSlider.value = Math.round(fixation * 100).toString();
		els.fixationValue.textContent = Math.round(fixation * 100) + '%';

		updateBionicPreview();
	}

	els.bionicToggle.addEventListener('change', () => {
		setBionicEnabled(els.bionicToggle.checked);
		updateBionicPreview();
	});

	els.fixationSlider.addEventListener('input', () => {
		const value = parseInt(els.fixationSlider.value, 10);
		els.fixationValue.textContent = value + '%';
		setBionicFixation(value / 100);
		updateBionicPreview();
	});

	initBionicSettings();
}
