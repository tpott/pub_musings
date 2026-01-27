#!/usr/bin/env python3
"""
buggy_calculator.py - Example script with intentional bugs for demonstrating automated debugging

This script has several bugs that can be found using automated debugging:
1. Division by zero when list is empty
2. Off-by-one error in median calculation
3. Logic error in mode calculation
"""

def calculate_mean(numbers):
    """Calculate arithmetic mean of a list of numbers."""
    total = sum(numbers)
    count = len(numbers)
    return total / count  # Bug: No check for empty list


def calculate_median(numbers):
    """Calculate median of a list of numbers."""
    sorted_nums = sorted(numbers)
    n = len(sorted_nums)
    
    if n % 2 == 0:
        # Bug: Off-by-one error
        middle_right = n / 2
        middle_left = middle_right - 1
        return (sorted_nums[int(middle_left)] + sorted_nums[int(middle_right)]) / 2
    else:
        middle = n // 2
        return sorted_nums[middle]


def calculate_mode(numbers):
    """Find the most common number in the list."""
    if not numbers:
        return None
    
    # Count occurrences
    counts = {}
    for num in numbers:
        counts[num] = counts.get(num, 0) + 1
    
    # Bug: Finding minimum instead of maximum
    most_common = min(counts.items(), key=lambda x: x[1])
    return most_common[0]


def analyze_dataset(data, name="Dataset"):
    """Analyze a dataset and print statistics."""
    print(f"\n{'='*50}")
    print(f"Analysis: {name}")
    print(f"{'='*50}")
    print(f"Data: {data}")
    print(f"Count: {len(data)}")
    
    try:
        mean = calculate_mean(data)
        print(f"Mean: {mean:.2f}")
    except Exception as e:
        print(f"Mean: Error - {e}")
    
    try:
        median = calculate_median(data)
        print(f"Median: {median:.2f}")
    except Exception as e:
        print(f"Median: Error - {e}")
    
    try:
        mode = calculate_mode(data)
        print(f"Mode: {mode}")
    except Exception as e:
        print(f"Mode: Error - {e}")


def main():
    """Main function to demonstrate the bugs."""
    
    # Test case 1: Normal data - mode calculation is wrong
    dataset1 = [1, 2, 2, 3, 3, 3, 4, 4, 5]
    analyze_dataset(dataset1, "Normal dataset")
    # Expected mode: 3 (appears 3 times)
    # Actual: Will return 1 (appears only 1 time) - BUG!
    
    # Test case 2: Even number of elements - median might be wrong
    dataset2 = [1, 2, 3, 4]
    analyze_dataset(dataset2, "Even-length dataset")
    # Expected median: 2.5
    # Actual: Might throw error or give wrong value - BUG!
    
    # Test case 3: Empty dataset - will crash
    dataset3 = []
    analyze_dataset(dataset3, "Empty dataset")
    # This will crash with ZeroDivisionError - BUG!
    
    print(f"\n{'='*50}")
    print("Analysis complete!")
    print(f"{'='*50}\n")


if __name__ == "__main__":
    main()
