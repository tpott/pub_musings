/**
 * History stack manager for undo/redo functionality.
 * Generic implementation that can work with any state type.
 */

/**
 * Deep clone an object, using structuredClone when available
 * and falling back to JSON for older browsers.
 *
 * Note: structuredClone handles more edge cases (undefined, circular refs)
 * but JSON fallback is sufficient for simple data structures like segments.
 */
export function deepClone<T>(value: T): T {
	if (typeof structuredClone === 'function') {
		return structuredClone(value);
	}
	// Fallback for older browsers
	return JSON.parse(JSON.stringify(value));
}

export interface HistoryManager<T> {
	push(state: T): void;
	undo(): T | null;
	redo(): T | null;
	canUndo(): boolean;
	canRedo(): boolean;
	clear(): void;
	getUndoCount(): number;
	getRedoCount(): number;
}

/**
 * Creates a history manager for tracking state changes.
 * @param maxSize Maximum number of history entries to keep (default 50)
 * @returns History manager instance
 */
export function createHistoryManager<T>(maxSize: number = 50): HistoryManager<T> {
	let undoStack: T[] = [];
	let redoStack: T[] = [];

	return {
		/**
		 * Push current state to history before making a change.
		 * Clears the redo stack since we're starting a new branch.
		 */
		push(state: T): void {
			// Deep copy the state to prevent reference issues
			const snapshot = deepClone(state);
			undoStack.push(snapshot);

			// Limit history size
			if (undoStack.length > maxSize) {
				undoStack.shift();
			}

			// Clear redo stack when new edit is made
			redoStack = [];
		},

		/**
		 * Undo the last change. Returns the previous state or null if nothing to undo.
		 * @param currentState The current state to push to redo stack
		 */
		undo(): T | null {
			if (undoStack.length === 0) return null;
			return undoStack.pop()!;
		},

		/**
		 * Redo a previously undone change. Returns the next state or null if nothing to redo.
		 */
		redo(): T | null {
			if (redoStack.length === 0) return null;
			return redoStack.pop()!;
		},

		/**
		 * Save state to redo stack (call before undo restores state)
		 */
		canUndo(): boolean {
			return undoStack.length > 0;
		},

		canRedo(): boolean {
			return redoStack.length > 0;
		},

		clear(): void {
			undoStack = [];
			redoStack = [];
		},

		getUndoCount(): number {
			return undoStack.length;
		},

		getRedoCount(): number {
			return redoStack.length;
		},
	};
}

/**
 * Extended history manager that also tracks current state for proper undo/redo flow.
 */
export function createStatefulHistoryManager<T>(maxSize: number = 50): HistoryManager<T> & {
	pushToRedo(state: T): void;
} {
	let undoStack: T[] = [];
	let redoStack: T[] = [];

	return {
		push(state: T): void {
			const snapshot = deepClone(state);
			undoStack.push(snapshot);
			if (undoStack.length > maxSize) {
				undoStack.shift();
			}
			redoStack = [];
		},

		pushToRedo(state: T): void {
			const snapshot = deepClone(state);
			redoStack.push(snapshot);
		},

		undo(): T | null {
			if (undoStack.length === 0) return null;
			return undoStack.pop()!;
		},

		redo(): T | null {
			if (redoStack.length === 0) return null;
			return redoStack.pop()!;
		},

		canUndo(): boolean {
			return undoStack.length > 0;
		},

		canRedo(): boolean {
			return redoStack.length > 0;
		},

		clear(): void {
			undoStack = [];
			redoStack = [];
		},

		getUndoCount(): number {
			return undoStack.length;
		},

		getRedoCount(): number {
			return redoStack.length;
		},
	};
}
