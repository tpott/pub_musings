/**
 * Collapsible section toggle for the upload/edit page.
 * Manages collapsed/expanded state with localStorage persistence.
 */

export function setupCollapsible(
	section: HTMLElement,
	header: HTMLElement,
	storageKey: string,
	defaultCollapsed: boolean
): void {
	const savedState = localStorage.getItem(storageKey);
	const isCollapsed = savedState !== null ? savedState === 'true' : defaultCollapsed;

	if (isCollapsed) {
		section.classList.add('collapsed');
		header.setAttribute('aria-expanded', 'false');
	} else {
		section.classList.remove('collapsed');
		header.setAttribute('aria-expanded', 'true');
	}

	const toggle = () => {
		const collapsed = section.classList.contains('collapsed');
		if (collapsed) {
			section.classList.remove('collapsed');
			header.setAttribute('aria-expanded', 'true');
			localStorage.setItem(storageKey, 'false');
		} else {
			section.classList.add('collapsed');
			header.setAttribute('aria-expanded', 'false');
			localStorage.setItem(storageKey, 'true');
		}
	};

	header.addEventListener('click', toggle);
	header.addEventListener('keydown', (e) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			toggle();
		}
	});
}
