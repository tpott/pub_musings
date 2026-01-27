#!/usr/bin/env python3
"""
A simple statistics calculator with a subtle bug.

This program is designed to demonstrate automated pdb debugging.
It calculates various statistics for a list of numbers, but one
of the calculations has a bug that's not obvious from reading the code.

Expected output for [10, 20, 30, 40, 50]:
- Sum: 150
- Count: 5
- Average: 30.0
- Variance: 200.0

But something is wrong...
"""


def calculate_stats(numbers):
    """Calculate sum, count, average, and variance for a list of numbers."""
    if not numbers:
        return {"sum": 0, "count": 0, "average": 0, "variance": 0}

    total = 0
    count = 0

    for num in numbers:
        total += num
        count += 1

    average = total / count

    # Calculate variance: sum of squared differences from mean, divided by count
    squared_diff_sum = 0
    for num in numbers:
        diff = num - average
        squared_diff_sum += diff * diff

    # Bug is here - using wrong variable
    variance = squared_diff_sum / total

    return {
        "sum": total,
        "count": count,
        "average": average,
        "variance": variance
    }


def main():
    test_data = [10, 20, 30, 40, 50]

    print(f"Input data: {test_data}")
    print()

    stats = calculate_stats(test_data)

    print("Calculated statistics:")
    print(f"  Sum:      {stats['sum']}")
    print(f"  Count:    {stats['count']}")
    print(f"  Average:  {stats['average']}")
    print(f"  Variance: {stats['variance']}")

    # Verify the results
    print()
    print("Expected values:")
    print("  Sum:      150")
    print("  Count:    5")
    print("  Average:  30.0")
    print("  Variance: 200.0")

    # Check if variance is correct
    expected_variance = 200.0
    if abs(stats['variance'] - expected_variance) > 0.001:
        print()
        print(f"ERROR: Variance is wrong! Got {stats['variance']}, expected {expected_variance}")
        return 1

    print()
    print("All calculations correct!")
    return 0


if __name__ == '__main__':
    exit(main())
