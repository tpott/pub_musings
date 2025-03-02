# loop.py
# Mon Feb 24 20:44:17 PST 2025

from pricechecker import priceSummaries


def main():
    """Loop to continuously prompt the user, query ChatGPT, and process responses."""
    print("ChatGPT Interactive (type 'exit' to quit)\n")
    
    while True:
        # Get user input
        try:
            user_input = input("product? ")
        except EOFError:
            print("Exiting")
            break
        
        # Exit condition
        if user_input.lower() in ["exit", "quit"]:
            print("Goodbye!")
            break

        priceSumary = priceSummaries(user_input)
       

if __name__ == "__main__":
    main()

