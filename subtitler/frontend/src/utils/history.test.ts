import { describe, it, expect, beforeEach, vi } from 'vitest';
import { createHistoryManager, createStatefulHistoryManager, deepClone } from './history';

describe('deepClone', () => {
	it('should deep clone simple objects', () => {
		const original = { a: 1, b: 'test', c: true };
		const cloned = deepClone(original);

		expect(cloned).toEqual(original);
		expect(cloned).not.toBe(original);
	});

	it('should deep clone nested objects', () => {
		const original = { outer: { inner: { value: 42 } } };
		const cloned = deepClone(original);

		expect(cloned).toEqual(original);
		expect(cloned.outer).not.toBe(original.outer);
		expect(cloned.outer.inner).not.toBe(original.outer.inner);
	});

	it('should deep clone arrays', () => {
		const original = [1, 2, { a: 3 }];
		const cloned = deepClone(original);

		expect(cloned).toEqual(original);
		expect(cloned).not.toBe(original);
		expect(cloned[2]).not.toBe(original[2]);
	});

	it('should prevent modifications to original from affecting clone', () => {
		const original = { data: [{ id: 1, text: 'hello' }] };
		const cloned = deepClone(original);

		original.data[0].text = 'modified';

		expect(cloned.data[0].text).toBe('hello');
	});

	it('should handle null and primitive values', () => {
		expect(deepClone(null)).toBeNull();
		expect(deepClone(42)).toBe(42);
		expect(deepClone('string')).toBe('string');
		expect(deepClone(true)).toBe(true);
	});

	it('should fall back to JSON when structuredClone is unavailable', () => {
		// Save original structuredClone
		const originalStructuredClone = globalThis.structuredClone;

		// Remove structuredClone to test fallback
		// @ts-expect-error - intentionally removing for test
		delete globalThis.structuredClone;

		try {
			const original = { test: 'value', nested: { arr: [1, 2, 3] } };
			const cloned = deepClone(original);

			expect(cloned).toEqual(original);
			expect(cloned).not.toBe(original);
		} finally {
			// Restore structuredClone
			globalThis.structuredClone = originalStructuredClone;
		}
	});
});

interface TestSegment {
	id: number;
	text: string;
	start: number;
	end: number;
}

describe('createHistoryManager', () => {
	let history: ReturnType<typeof createHistoryManager<TestSegment[]>>;

	beforeEach(() => {
		history = createHistoryManager<TestSegment[]>(50);
	});

	describe('initial state', () => {
		it('should start with empty stacks', () => {
			expect(history.canUndo()).toBe(false);
			expect(history.canRedo()).toBe(false);
			expect(history.getUndoCount()).toBe(0);
			expect(history.getRedoCount()).toBe(0);
		});

		it('should return null when undoing empty stack', () => {
			expect(history.undo()).toBeNull();
		});

		it('should return null when redoing empty stack', () => {
			expect(history.redo()).toBeNull();
		});
	});

	describe('push', () => {
		it('should add state to undo stack', () => {
			const state: TestSegment[] = [{ id: 0, text: 'Hello', start: 0, end: 1 }];
			history.push(state);

			expect(history.canUndo()).toBe(true);
			expect(history.getUndoCount()).toBe(1);
		});

		it('should clear redo stack when pushing new state', () => {
			const state1: TestSegment[] = [{ id: 0, text: 'First', start: 0, end: 1 }];
			const state2: TestSegment[] = [{ id: 0, text: 'Second', start: 0, end: 1 }];
			const state3: TestSegment[] = [{ id: 0, text: 'Third', start: 0, end: 1 }];

			history.push(state1);
			history.push(state2);
			history.undo(); // Move state2 conceptually available for redo

			// Push new state should clear redo
			history.push(state3);
			expect(history.canRedo()).toBe(false);
		});

		it('should deep copy state to prevent reference issues', () => {
			const state: TestSegment[] = [{ id: 0, text: 'Original', start: 0, end: 1 }];
			history.push(state);

			// Modify original state
			state[0].text = 'Modified';

			// Undo should return the original value
			const restored = history.undo();
			expect(restored![0].text).toBe('Original');
		});

		it('should limit history size', () => {
			const smallHistory = createHistoryManager<TestSegment[]>(3);

			for (let i = 0; i < 5; i++) {
				smallHistory.push([{ id: i, text: `State ${i}`, start: 0, end: 1 }]);
			}

			expect(smallHistory.getUndoCount()).toBe(3);
		});
	});

	describe('undo', () => {
		it('should return previous state', () => {
			const state1: TestSegment[] = [{ id: 0, text: 'First', start: 0, end: 1 }];
			const state2: TestSegment[] = [{ id: 0, text: 'Second', start: 0, end: 2 }];

			history.push(state1);
			history.push(state2);

			const restored = history.undo();
			expect(restored![0].text).toBe('Second');
		});

		it('should decrease undo count', () => {
			history.push([{ id: 0, text: 'Test', start: 0, end: 1 }]);
			history.push([{ id: 0, text: 'Test2', start: 0, end: 2 }]);

			expect(history.getUndoCount()).toBe(2);
			history.undo();
			expect(history.getUndoCount()).toBe(1);
		});
	});

	describe('clear', () => {
		it('should clear both stacks', () => {
			history.push([{ id: 0, text: 'Test', start: 0, end: 1 }]);
			history.push([{ id: 0, text: 'Test2', start: 0, end: 2 }]);
			history.undo();

			history.clear();

			expect(history.canUndo()).toBe(false);
			expect(history.canRedo()).toBe(false);
		});
	});
});

describe('createStatefulHistoryManager', () => {
	let history: ReturnType<typeof createStatefulHistoryManager<TestSegment[]>>;

	beforeEach(() => {
		history = createStatefulHistoryManager<TestSegment[]>(50);
	});

	describe('pushToRedo', () => {
		it('should add state to redo stack', () => {
			const state: TestSegment[] = [{ id: 0, text: 'Current', start: 0, end: 1 }];
			history.pushToRedo(state);

			expect(history.canRedo()).toBe(true);
			expect(history.getRedoCount()).toBe(1);
		});

		it('should deep copy state', () => {
			const state: TestSegment[] = [{ id: 0, text: 'Original', start: 0, end: 1 }];
			history.pushToRedo(state);

			state[0].text = 'Modified';

			const restored = history.redo();
			expect(restored![0].text).toBe('Original');
		});
	});

	describe('full undo/redo flow', () => {
		it('should support undo then redo', () => {
			// Simulate editing flow:
			// Edit 1: Add "Hello"
			// Edit 2: Change to "Hello World"

			const state1: TestSegment[] = [{ id: 0, text: 'Hello', start: 0, end: 1 }];
			const state2: TestSegment[] = [{ id: 0, text: 'Hello World', start: 0, end: 2 }];

			// Push state before edit 2
			history.push(state1);

			// Current state is state2, undo stack has: [state1]

			// Undo: save state2 to redo, restore state1
			history.pushToRedo(state2);
			const undo1 = history.undo();
			expect(undo1![0].text).toBe('Hello');
			expect(history.canRedo()).toBe(true);

			// Redo: restore state2 from redo
			const redo1 = history.redo();
			expect(redo1![0].text).toBe('Hello World');
		});

		it('should clear redo when new edit is made after undo', () => {
			const state1: TestSegment[] = [{ id: 0, text: 'Hello', start: 0, end: 1 }];
			const state2: TestSegment[] = [{ id: 0, text: 'Hello World', start: 0, end: 2 }];
			const state3: TestSegment[] = [{ id: 0, text: 'Hello Again', start: 0, end: 3 }];

			history.push(state1);
			history.pushToRedo(state2);
			history.undo(); // Now at state1, redo has state2

			// Make a new edit (push clears redo)
			history.push(state1); // Before making change to state3

			expect(history.canRedo()).toBe(false);
		});

		it('should handle multiple undos', () => {
			const state1: TestSegment[] = [{ id: 0, text: 'First', start: 0, end: 1 }];
			const state2: TestSegment[] = [{ id: 0, text: 'Second', start: 0, end: 2 }];
			const state3: TestSegment[] = [{ id: 0, text: 'Third', start: 0, end: 3 }];

			history.push(state1);
			history.push(state2);
			// Current is state3, undo has [state1, state2]

			// Undo to state2
			history.pushToRedo(state3);
			const undo1 = history.undo();
			expect(undo1![0].text).toBe('Second');

			// Undo to state1
			history.pushToRedo(undo1!);
			const undo2 = history.undo();
			expect(undo2![0].text).toBe('First');

			expect(history.canUndo()).toBe(false);
			expect(history.getRedoCount()).toBe(2);
		});
	});

	describe('segment operations', () => {
		it('should handle segment deletion undo', () => {
			const withSegment: TestSegment[] = [
				{ id: 0, text: 'Keep me', start: 0, end: 1 },
				{ id: 1, text: 'Delete me', start: 1, end: 2 },
			];
			const afterDelete: TestSegment[] = [
				{ id: 0, text: 'Keep me', start: 0, end: 1 },
			];

			// Push state before deletion
			history.push(withSegment);

			// Undo deletion should restore deleted segment
			const restored = history.undo();
			expect(restored).toHaveLength(2);
			expect(restored![1].text).toBe('Delete me');
		});

		it('should handle segment addition undo', () => {
			const before: TestSegment[] = [
				{ id: 0, text: 'Original', start: 0, end: 1 },
			];
			const after: TestSegment[] = [
				{ id: 0, text: 'Original', start: 0, end: 1 },
				{ id: 1, text: 'New segment', start: 1, end: 4 },
			];

			// Push state before addition
			history.push(before);

			// Undo addition should remove new segment
			const restored = history.undo();
			expect(restored).toHaveLength(1);
		});

		it('should handle timing changes undo', () => {
			const before: TestSegment[] = [
				{ id: 0, text: 'Test', start: 0, end: 1 },
			];
			const after: TestSegment[] = [
				{ id: 0, text: 'Test', start: 0.5, end: 2.5 },
			];

			history.push(before);

			const restored = history.undo();
			expect(restored![0].start).toBe(0);
			expect(restored![0].end).toBe(1);
		});
	});
});
