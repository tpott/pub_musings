# mealprep.py

import argparse
import json
import os
from typing import (Dict, List)

import requests


def chatCompletitions(messages: List[Dict[str, str]]) -> str:
    api_key_file = os.environ.get("OPENAI_API_KEY_FILE")
    assert api_key_file is not None, "Missing env var: OPENAI_API_KEY_FILE"
    openai_api_key = open(api_key_file).read().strip()
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {openai_api_key}",
    }
    data = {
        "model": "gpt-3.5-turbo",
        "messages": messages,
    }
    resp = requests.post("https://api.openai.com/v1/chat/completions", headers=headers, data=json.dumps(data))
    resp.raise_for_status()
    print(resp.json())
    print(resp.json()['usage'])
    return resp.json()['choices'][0]['message']['content']


def main() -> None:
    parser = argparse.ArgumentParser(description="Meal prep planning")
    parser.add_argument("ingredient_list", help="Path to meal ingredients. START HERE!")
    parser.add_argument("meal_list", help="Path to meal list file. Doesn't need to exist.")
    parser.add_argument("meal_plan", help="Path to this week's meal plan file. Doesn't need to exist.")
    parser.add_argument("modifications", help="Optional newline (\"\\n\") separated list of modifications", nargs="?")
    args = parser.parse_args()

    api_key_file = os.environ.get("OPENAI_API_KEY_FILE")
    assert api_key_file is not None, "Missing env var: OPENAI_API_KEY_FILE"

    # TODO check facebook messages in the last week
    # group messages according to week's plan
    # add or remove items to a week's plan
    # print list of things to shop for, ordered by store layout
    # add or remove items that we already have

    if not os.path.isfile(args.meal_list):
        # help construct the meal list
        print(chatCompletitions([
            # This just kinda sorta reformatted the list
            # {"role": "system", "content": "Please help me summarize these meals and ingredients into a list of meal options"},
            # There's still a lot of "{something} _with_ {ingredients}"
            # {"role": "system", "content": "Please help me summarize these meals and ingredients into a list of meal options. Don't list individual ingredients. Summarize each meal or snack into a couple word description or title."},
            # There's _still_ a lot of "{something} _with_ {ingredients}"
            {"role": "system", "content": "Please help me summarize these meals and ingredients into a list of meal options. Don't list individual ingredients. Summarize each meal or snack into a couple word description or title. If you find yourself saying something \"with\" some ingredients, please don't list the ingredients."},
            # TODO escape ingredient_list file content
            {"role": "user", "content": open(args.ingredient_list).read()},
        ]))
        return

    if not os.path.isfile(args.meal_plan):
        # help prototype the meal plan for the week
        print(chatCompletitions([
            # Days were numbered, rather than named.
            # {"role": "system", "content": "Please help me plan a week's worth of meals given these meal options."},
            # Not enough specific lunch or dinners
            # {"role": "system", "content": "Please help me plan a week's worth of meals given these meal options. Start the week on Monday."},
            # It really doesn't know how to include snacks
            # {"role": "system", "content": "Please help me plan a week's worth of meals given these meal options. Start the week on Monday. Each day should include a breakfast, lunch and dinner. Each day may include an optional afternoon snack."},
            {"role": "system", "content": "Please help me plan a week's worth of meals given these meal options. Start the week on Monday. Only plan for the work week, Monday through Friday. Each day should include a breakfast, lunch and dinner."},
            # TODO escape meal_list file content
            {"role": "user", "content": open(args.meal_list).read()},
        ]))
        return

    if args.modifications is not None and args.modifications != "":
        messages = [
            {"role": "system", "content": f"Please help me plan a week's worth of meals given these meal options. Start the week on Monday. Only plan for the work week, Monday through Friday. Each day should include a breakfast, lunch and dinner. The current plan is: {open(args.meal_plan).read()}"},
        ]
        for message in args.modifications.split("\n"):
            messages.append({"role": "user", "content": message})
        print(chatCompletitions(messages))
        return

    # aka "print shopping list"
    # help construct the meal list
    print(chatCompletitions([
        {"role": "system", "content": "Please help sort the following recipe ingredients for the following meals for monday - friday. First start with fruits and vegetables, second meats, third dairies and sauces, and finally any grains."},
        # TODO escape ingredient_list file content
        {"role": "user", "content": open(args.ingredient_list).read()},
        # TODO escape meal_plan file content
        {"role": "user", "content": open(args.meal_plan).read()},
    ]))
    return

    print("There were no modifications to the meal_plan. Ship it!")


if __name__ == "__main__":
    main()
