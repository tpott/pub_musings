# Agent Instructions

## Commands

### Run Tests

```bash
cd ralph && python -m unittest discover tests/ -v
```

### Format Code

```bash
cd ralph && python -m black src/ tests/
```

### Type Check

```bash
cd ralph && python -m mypy --strict src/ tests/
```

All three must pass before submitting changes.
