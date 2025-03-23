#!/usr/bin/env python3

# A simple Python script that:
#   1. Reads Google and OpenAI API keys from files specified by env vars:
#      - GCLOUD_API_KEY_FILE
#      - OPENAI_API_KEY_FILE
#   2. Validates file permissions (must be 0o400 or 0o600)
#   3. Uses an argparse "verbose" flag (-v) to optionally print API requests
#   4. Uses OpenAI Chat Completions to parse user input into "tool calls"
#   5. Calls the Google Places (Nearby Search) and Distance Matrix APIs
#   6. Remembers locations, plans routes, offers help
#   7. Saves conversation lines to a file in append-only format
#   8. Loops until Ctrl+D or Ctrl+C (or user types exit/quit)

import os
import sys
import json
import stat
import argparse
import requests
from typing import Any, Dict, List


def check_file_permissions(path: str) -> None:
    """
    Check that the file at 'path' has permissions 0o400 or 0o600.
    Exit if not.
    """
    try:
        mode = os.stat(path).st_mode
        perms = mode & 0o777
    except Exception as e:
        print("Cannot stat file:", path, str(e))
        sys.exit(1)

    if perms != 0o400 and perms != 0o600:
        msg = (
            "File '"
            + path
            + "' must have permissions 400 or 600, found "
            + oct(perms)
            + "."
        )
        print(msg)
        sys.exit(1)


def read_api_keys(
    google_key_file: str,
    openai_key_file: str,
) -> Dict[str, str]:
    """
    Reads Google Maps and OpenAI API keys from the given file paths.
    Validates file permissions (0o400 or 0o600).
    Exits on failure.
    """
    check_file_permissions(google_key_file)
    check_file_permissions(openai_key_file)

    try:
        with open(google_key_file, "r") as fg:
            gkey = fg.read().strip()
        with open(openai_key_file, "r") as fo:
            okey = fo.read().strip()
    except Exception as e:
        print("Error reading key files:", str(e))
        sys.exit(1)

    return {"google_api_key": gkey, "openai_api_key": okey}


def tool_chat_completion(
    openai_api_key: str,
    messages: List[Dict[str, str]],
    verbose: bool,
) -> Dict[str, Any]:
    """
    Calls the OpenAI Chat Completions API.
    Returns {"content": "..."} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if openai_api_key == "":
        out["error"] = "Missing openai_api_key"
        return out

    url = "https://api.openai.com/v1/chat/completions"
    headers = {
        "Authorization": "Bearer " + openai_api_key,
        "Content-Type": "application/json",
    }
    payload = {
        "model": "gpt-3.5-turbo",
        "messages": messages,
        "temperature": 0.0,
    }

    if verbose is True:
        print("OpenAI POST:", url)

    try:
        resp = requests.post(url, headers=headers, json=payload)
    except Exception as e:
        out["error"] = "OpenAI request failed: " + str(e)
        return out

    if resp.status_code != 200:
        out["error"] = (
            "OpenAI API error "
            + str(resp.status_code)
            + ": "
            + resp.text
        )
        return out

    data = resp.json()
    choices = data.get("choices", [])
    if len(choices) < 1:
        out["error"] = "No completion choices returned."
        return out

    content = choices[0].get("message", {}).get("content", "")
    out["content"] = content
    return out


def tool_nearby_search(
    google_api_key: str,
    location: str,
    place_type: str,
    radius: int,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Places Nearby Search call.
    Returns {"results": [...]} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if google_api_key == "" or location == "":
        out["error"] = "Missing google_api_key or location"
        return out

    url = "https://maps.googleapis.com/maps/api/place/nearbysearch/json"
    params = {
        "key": google_api_key,
        "location": location,
        "type": place_type,
        "radius": radius,
    }

    if verbose is True:
        print("Places GET:", url, params)

    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = "Nearby Search request failed: " + str(e)
        return out

    if resp.status_code != 200:
        out["error"] = (
            "Nearby Search error "
            + str(resp.status_code)
            + ", text: "
            + resp.text
        )
        return out

    data = resp.json()
    out["results"] = data.get("results", [])
    return out


def tool_get_distance_matrix(
    google_api_key: str,
    origin: str,
    destination: str,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Distance Matrix call.
    Returns distance/duration fields or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if google_api_key == "" or origin == "" or destination == "":
        out["error"] = "Missing google_api_key/origin/destination"
        return out

    url = "https://maps.googleapis.com/maps/api/distancematrix/json"
    params = {"key": google_api_key, "origins": origin, "destinations": destination}

    if verbose is True:
        print("DistanceMatrix GET:", url, params)

    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = "Distance Matrix request failed: " + str(e)
        return out

    if resp.status_code != 200:
        out["error"] = (
            "Distance Matrix error "
            + str(resp.status_code)
            + ", text: "
            + resp.text
        )
        return out

    data = resp.json()
    rows = data.get("rows", [])
    if len(rows) < 1:
        out["error"] = "No rows in Distance Matrix response"
        return out

    elems = rows[0].get("elements", [])
    if len(elems) < 1:
        out["error"] = "No elements in Distance Matrix row"
        return out

    dist = elems[0].get("distance", {})
    dur = elems[0].get("duration", {})
    out["distance_text"] = dist.get("text", "")
    out["distance_value"] = dist.get("value", 0)
    out["duration_text"] = dur.get("text", "")
    out["duration_value"] = dur.get("value", 0)
    return out


def tool_help() -> None:
    """
    Prints help info with example commands.
    """
    print("Commands you can try:")
    print("  search for coffee near seattle")
    print("  remember home is 40.7128,-74.0060")
    print("  plan route from home to seattle")
    print("  exit or quit")


def save_chat_line(role: str, content: str, filename: str) -> None:
    """
    Appends a single JSON line with 'role' and 'content' to 'filename'.
    """
    record = {"role": role, "content": content}
    try:
        with open(filename, "a", encoding="utf-8") as f:
            f.write(json.dumps(record))
            f.write("\n")
    except Exception as e:
        print("Could not save chat line:", str(e))


def handle_search(
    google_api_key: str,
    location: str,
    place_type: str,
    verbose: bool,
) -> None:
    radius = 5000
    res = tool_nearby_search(google_api_key, location, place_type, radius, verbose)
    if "error" in res:
        print("Search error:", res["error"])
        return

    results = res.get("results", [])
    if len(results) < 1:
        print("No places found.")
        return

    limit = 3
    count = min(limit, len(results))
    for i in range(count):
        place = results[i]
        name = place.get("name", "?")
        vicinity = place.get("vicinity", "?")
        index = i + 1
        print(str(index) + ". " + name + " - " + vicinity)


def handle_remember(
    known_locations: Dict[str, str],
    label: str,
    location: str,
) -> None:
    if label == "" or location == "":
        print("Invalid remember arguments.")
        return

    known_locations[label] = location
    print("Remembered '" + label + "' as '" + location + "'.")


def handle_plan_route(
    google_api_key: str,
    known_locations: Dict[str, str],
    frm: str,
    to: str,
    verbose: bool,
) -> None:
    if frm not in known_locations or to not in known_locations:
        print("Unknown label(s). Use 'remember' first.")
        return

    origin_val = known_locations[frm]
    dest_val = known_locations[to]
    res = tool_get_distance_matrix(google_api_key, origin_val, dest_val, verbose)
    if "error" in res:
        print("Route error:", res["error"])
        return

    dist_txt = res["distance_text"]
    dur_txt = res["duration_text"]
    print("Route from '" + frm + "' to '" + to + "': " + dist_txt + " in " + dur_txt)


def conversation_loop(
    google_api_key: str,
    openai_api_key: str,
    verbose: bool,
    history_file: str,
) -> None:
    known_locations: Dict[str, str] = {}
    conversation: List[Dict[str, str]] = []
    system_prompt = (
        "You are a helpful AI. You read the user's input and produce JSON "
        "for the relevant tool call, e.g. "
        '{"action":"search","location":"seattle","type":"coffee"}. '
        "If the user is not requesting a known action, respond with "
        '{"action":"help"}. "'
    )

    conversation.append({"role": "system", "content": system_prompt})
    save_chat_line("system", system_prompt, history_file)

    while True:
        try:
            line = input("> ")
        except (EOFError, KeyboardInterrupt):
            print("\nExiting.")
            break

        line = line.strip()
        if line.lower() == "exit" or line.lower() == "quit":
            print("Exiting.")
            break

        conversation.append({"role": "user", "content": line})
        save_chat_line("user", line, history_file)

        c_out = tool_chat_completion(openai_api_key, conversation, verbose)
        if "error" in c_out:
            print("OpenAI error:", c_out["error"])
            continue

        content = c_out.get("content", "")
        conversation.append({"role": "assistant", "content": content})
        save_chat_line("assistant", content, history_file)

        try:
            parsed = json.loads(content)
        except Exception:
            print("Assistant returned non-JSON content. Raw output:")
            print(content)
            continue

        action = parsed.get("action", "")
        if action == "help":
            tool_help()
            continue

        if action == "search":
            loc = parsed.get("location", "")
            typ = parsed.get("type", "restaurant")
            handle_search(google_api_key, loc, typ, verbose)
            continue

        if action == "remember":
            lbl = parsed.get("label", "")
            lct = parsed.get("location", "")
            handle_remember(known_locations, lbl, lct)
            continue

        if action == "plan_route":
            frm = parsed.get("from_label", "")
            to = parsed.get("to_label", "")
            handle_plan_route(google_api_key, known_locations, frm, to, verbose)
            continue

        print("No recognized action. Raw output:")
        print(content)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Simple script to chat & call Google Maps APIs."
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Print verbose info (API URLs and params).",
    )
    parser.add_argument(
        "--history-file",
        default="conversation.log",
        help="File to append conversation lines to.",
    )
    args = parser.parse_args()

    google_path = os.environ.get("GCLOUD_API_KEY_FILE", "")
    openai_path = os.environ.get("OPENAI_API_KEY_FILE", "")
    if google_path == "" or openai_path == "":
        print("Need GCLOUD_API_KEY_FILE and OPENAI_API_KEY_FILE environment vars.")
        sys.exit(1)

    keys = read_api_keys(google_path, openai_path)
    gkey = keys["google_api_key"]
    okey = keys["openai_api_key"]

    conversation_loop(
        google_api_key=gkey,
        openai_api_key=okey,
        verbose=args.verbose,
        history_file=args.history_file,
    )


if __name__ == "__main__":
    main()
