#!/usr/bin/env python3

import os
import sys
import json
import stat
import argparse
import requests
from typing import Any, Dict, List

def check_file_permissions(path: str) -> None:
    """
    Ensure that 'path' has mode 0o400 or 0o600. Exit if not.
    """
    try:
        mode = os.stat(path).st_mode
        perms = mode & 0o777
    except Exception as e:
        print(f"Cannot stat file {path}: {e}")
        sys.exit(1)
    if perms != 0o400 and perms != 0o600:
        msg = (
            f"File '{path}' must have permissions 400 or 600, but has {oct(perms)}. "
            "Please run chmod 400 or chmod 600 on the file."
        )
        print(msg)
        sys.exit(1)

def read_api_keys(google_key_file: str, openai_key_file: str) -> Dict[str, str]:
    """
    Read and return Google/OpenAI API keys from files after permission checks.
    """
    check_file_permissions(google_key_file)
    check_file_permissions(openai_key_file)
    try:
        with open(google_key_file, encoding="utf-8") as fg:
            gkey = fg.read().strip()
        with open(openai_key_file, encoding="utf-8") as fo:
            okey = fo.read().strip()
    except Exception as e:
        print(f"Error reading key files: {e}")
        sys.exit(1)
    return {"google_api_key": gkey, "openai_api_key": okey}

def save_chat_line(role: str, content: str, filename: str) -> None:
    """
    Append a JSON line with 'role' and 'content' to 'filename'.
    """
    record = {"role": role, "content": content}
    try:
        with open(filename, "a", encoding="utf-8") as f:
            f.write(json.dumps(record) + "\n")
    except Exception as e:
        print(f"Could not save chat line: {e}")

def tool_chat_completion(
    openai_api_key: str,
    messages: List[Dict[str, str]],
    verbose: bool,
) -> Dict[str, Any]:
    """
    Tool: Chat completion via OpenAI. 
    Expects messages; returns {"content": "..."} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if openai_api_key == "":
        out["error"] = "Missing openai_api_key"
        return out
    url = "https://api.openai.com/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {openai_api_key}",
        "Content-Type": "application/json",
    }
    payload = {"model": "gpt-3.5-turbo", "messages": messages, "temperature": 0.0}
    if verbose:
        print(f"OpenAI POST: {url}")
    try:
        resp = requests.post(url, headers=headers, json=payload)
    except Exception as e:
        out["error"] = f"OpenAI request failed: {e}"
        return out
    if resp.status_code != 200:
        out["error"] = f"OpenAI API error {resp.status_code}: {resp.text}"
        return out
    data = resp.json()
    choices = data.get("choices", [])
    if len(choices) < 1:
        out["error"] = "No completion choices returned."
        return out
    out["content"] = choices[0].get("message", {}).get("content", "")
    return out

def tool_geocode_address(google_api_key: str, address: str, verbose: bool) -> Dict[str, Any]:
    """
    Tool: Geocode an address. 
    Returns {"lat": float, "lng": float} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if google_api_key == "" or address == "":
        out["error"] = "Missing google_api_key or address"
        return out
    url = "https://maps.googleapis.com/maps/api/geocode/json"
    params = {"key": google_api_key, "address": address}
    if verbose:
        print(f"Geocode GET: {url} {params}")
    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = f"Geocoding request failed: {e}"
        return out
    if resp.status_code != 200:
        out["error"] = f"Geocoding error {resp.status_code}, text: {resp.text}"
        return out
    data = resp.json()
    status = data.get("status", "")
    if status != "OK":
        out["error"] = f"Geocoding API status: {status}, text: {resp.text}"
        return out
    results = data.get("results", [])
    if len(results) == 0:
        out["error"] = f"No geocoding results for '{address}'"
        return out
    loc = results[0].get("geometry", {}).get("location", {})
    out["lat"] = loc.get("lat", 0.0)
    out["lng"] = loc.get("lng", 0.0)
    return out

def tool_text_search(google_api_key: str, query: str, verbose: bool) -> Dict[str, Any]:
    """
    Tool: Google Places Text Search.
    Returns {"results": [...]} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if google_api_key == "" or query == "":
        out["error"] = "Missing google_api_key or query"
        return out
    url = "https://maps.googleapis.com/maps/api/place/textsearch/json"
    params = {"key": google_api_key, "query": query}
    if verbose:
        print(f"TextSearch GET: {url} {params}")
    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = f"Text Search request failed: {e}"
        return out
    if resp.status_code != 200:
        out["error"] = f"Text Search error {resp.status_code}, text: {resp.text}"
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
    Tool: Distance Matrix for route planning. 
    Returns distance/duration or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if google_api_key == "" or origin == "" or destination == "":
        out["error"] = "Missing google_api_key/origin/destination"
        return out
    url = "https://maps.googleapis.com/maps/api/distancematrix/json"
    params = {"key": google_api_key, "origins": origin, "destinations": destination}
    if verbose:
        print(f"DistanceMatrix GET: {url} {params}")
    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = f"Distance Matrix request failed: {e}"
        return out
    if resp.status_code != 200:
        out["error"] = f"Distance Matrix error {resp.status_code}, text: {resp.text}"
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

def tool_remember(
    known_locations: Dict[str, str],
    label: str,
    location: str,
) -> Dict[str, Any]:
    """
    Tool: Remember a location label. 
    Returns {"success": True} or {"error": "..."}.
    """
    out: Dict[str, Any] = {}
    if label == "" or location == "":
        out["error"] = "Empty label or location"
        return out
    known_locations[label] = location
    out["success"] = True
    return out

def tool_help() -> None:
    """
    Tool: Show help. 
    Lists all available tool actions with minimal parameters info.
    """
    print("Available tool actions:")
    print('  {"action":"help"}')
    print('  {"action":"text_search","query":"some query"}')
    print('  {"action":"geocode","address":"some address"}')
    print('  {"action":"distance_matrix","origin":"loc","destination":"loc"}')
    print('  {"action":"remember","label":"name","location":"lat,lng or text"}')
    print("You can also type 'exit' or 'quit' to end.")

def handle_chat_loop(
    google_api_key: str,
    openai_api_key: str,
    history_file: str,
    verbose: bool,
) -> None:
    known_locations: Dict[str, str] = {}
    convo: List[Dict[str, str]] = []
    sys_prompt = (
        "You are an assistant that outputs JSON for a relevant tool action, e.g. "
        '{"action":"text_search","query":"restaurants in Seattle"}. '
        'If input is not recognized, respond with {"action":"help"}.'
    )
    convo.append({"role": "system", "content": sys_prompt})
    save_chat_line("system", sys_prompt, history_file)
    while True:
        try:
            user_line = input("> ")
        except (EOFError, KeyboardInterrupt):
            print("\nExiting.")
            break
        user_line = user_line.strip()
        if user_line.lower() in ("exit", "quit"):
            print("Exiting.")
            break
        convo.append({"role": "user", "content": user_line})
        save_chat_line("user", user_line, history_file)
        completion = tool_chat_completion(openai_api_key, convo, verbose)
        if "error" in completion:
            print(f"OpenAI error: {completion['error']}")
            continue
        msg = completion.get("content", "")
        convo.append({"role": "assistant", "content": msg})
        save_chat_line("assistant", msg, history_file)
        try:
            parsed = json.loads(msg)
        except Exception:
            print("Assistant returned non-JSON content. Raw output:")
            print(msg)
            continue
        action = parsed.get("action", "")
        if action == "help":
            tool_help()
            continue
        if action == "text_search":
            query = parsed.get("query", "")
            resp = tool_text_search(google_api_key, query, verbose)
            if "error" in resp:
                print(f"Text Search error: {resp['error']}")
            else:
                results = resp.get("results", [])
                if not results:
                    print("No places found.")
                else:
                    limit = min(3, len(results))
                    for i in range(limit):
                        r = results[i]
                        name = r.get("name", "?")
                        address = r.get("formatted_address", "?")
                        print(f"{i+1}. {name} - {address}")
            continue
        if action == "geocode":
            address = parsed.get("address", "")
            geo = tool_geocode_address(google_api_key, address, verbose)
            if "error" in geo:
                print(f"Geocode error: {geo['error']}")
            else:
                lat = geo.get("lat", 0.0)
                lng = geo.get("lng", 0.0)
                print(f"Coordinates for '{address}': {lat},{lng}")
            continue
        if action == "distance_matrix":
            origin = parsed.get("origin", "")
            destination = parsed.get("destination", "")
            dist_data = tool_get_distance_matrix(google_api_key, origin, destination, verbose)
            if "error" in dist_data:
                print(f"Distance error: {dist_data['error']}")
            else:
                dtxt = dist_data["distance_text"]
                durtxt = dist_data["duration_text"]
                print(f"Distance from '{origin}' to '{destination}': {dtxt} in {durtxt}")
            continue
        if action == "remember":
            lbl = parsed.get("label", "")
            loc = parsed.get("location", "")
            result = tool_remember(known_locations, lbl, loc)
            if "error" in result:
                print(f"Remember error: {result['error']}")
            else:
                print(f"Remembered '{lbl}' as '{loc}'.")
            continue
        print("Unknown action. Raw output:")
        print(msg)

def main() -> None:
    parser = argparse.ArgumentParser(description="Chat-based Google Maps script.")
    parser.add_argument("-v", "--verbose", action="store_true", help="Print debug info.")
    parser.add_argument(
        "--history-file",
        default="conversation.log",
        help="File to append conversation lines to.",
    )
    args = parser.parse_args()
    google_file = os.environ.get("GCLOUD_API_KEY_FILE", "")
    openai_file = os.environ.get("OPENAI_API_KEY_FILE", "")
    if google_file == "" or openai_file == "":
        print("Must set GCLOUD_API_KEY_FILE and OPENAI_API_KEY_FILE environment vars.")
        sys.exit(1)
    keys = read_api_keys(google_file, openai_file)
    gkey = keys["google_api_key"]
    okey = keys["openai_api_key"]
    handle_chat_loop(gkey, okey, args.history_file, args.verbose)

if __name__ == "__main__":
    main()
