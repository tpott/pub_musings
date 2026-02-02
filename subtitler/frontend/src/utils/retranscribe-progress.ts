/**
 * Re-transcribe progress box management for the upload/edit page.
 * Shows a progress box with a bar, percentage, and status message
 * when re-transcription is in progress. Animates collapse on completion.
 */

export interface RetranscribeProgressElements {
	container: HTMLElement;
	fill: HTMLElement;
	pct: HTMLElement;
	msg: HTMLElement;
}

/** Show the progress box and reset its state */
export function showRetranscribeProgress(els: RetranscribeProgressElements): void {
	els.container.style.display = 'block';
	els.container.classList.remove('collapsing');
	els.fill.style.width = '0%';
	els.pct.textContent = '';
	els.msg.textContent = 'Starting...';
}

/** Update progress box from transcription status text */
export function updateRetranscribeProgress(
	els: RetranscribeProgressElements,
	text: string
): void {
	const pctMatch = text.match(/\((\d+)%\)/);
	if (pctMatch) {
		const pct = parseInt(pctMatch[1], 10);
		els.fill.style.width = `${pct}%`;
		els.pct.textContent = `${pct}%`;
	}
	els.msg.textContent = text;
}

/** Hide progress box immediately (on error) */
export function hideRetranscribeProgress(els: RetranscribeProgressElements): void {
	els.container.style.display = 'none';
}

/** Animate progress box collapse (on success) */
export function collapseRetranscribeProgress(els: RetranscribeProgressElements): void {
	els.msg.textContent = 'Complete!';
	els.fill.style.width = '100%';
	els.pct.textContent = '100%';

	setTimeout(() => {
		els.container.classList.add('collapsing');
		setTimeout(() => {
			els.container.style.display = 'none';
			els.container.classList.remove('collapsing');
		}, 500);
	}, 1500);
}
