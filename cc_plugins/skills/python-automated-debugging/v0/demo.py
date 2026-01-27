#!/usr/bin/env python3
"""
demo.py - Simple demo script for testing pdb debugging

This script has intentional bugs to demonstrate pdb debugging capabilities.
"""

def calculate_factorial(n):
    """Calculate factorial of n."""
    if n < 0:
        raise ValueError("Factorial is not defined for negative numbers")
    
    # Bug: doesn't handle n=0 correctly (should return 1)
    result = n
    for i in range(n-1, 1, -1):
        result *= i
    return result


def process_numbers(numbers):
    """Process a list of numbers and return their factorials."""
    results = []
    
    for num in numbers:
        print(f"Processing: {num}")
        
        # This is where you might set a breakpoint to inspect state
        # Uncomment the next line to add a breakpoint:
        # import pdb; pdb.set_trace()
        
        try:
            factorial = calculate_factorial(num)
            results.append({
                'number': num,
                'factorial': factorial
            })
        except ValueError as e:
            print(f"  Error: {e}")
            results.append({
                'number': num,
                'error': str(e)
            })
    
    return results


def main():
    """Main function."""
    print("Factorial Calculator Demo")
    print("=" * 40)
    
    # Test data with various cases
    test_numbers = [5, 3, 0, -1, 7]
    
    print(f"\nCalculating factorials for: {test_numbers}")
    print()
    
    results = process_numbers(test_numbers)
    
    print("\n" + "=" * 40)
    print("Results:")
    for item in results:
        if 'factorial' in item:
            print(f"  {item['number']}! = {item['factorial']}")
        else:
            print(f"  {item['number']}: {item['error']}")


if __name__ == "__main__":
    main()
