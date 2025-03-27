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
    Check that 'path' has mode 0o400 or 0o600. Exit if not.
    """
    try:
        mode = os.stat(path).st_mode
        perms = mode & 0o777
    except Exception as e:
        print(f"Cannot stat file '{path}': {e}")
        sys.exit(1)
    if perms != 0o400 and perms != 0o600:
        print(
            f"File '{path}' must have permissions 400 or 600; found {oct(perms)}. "
            "Please adjust permissions accordingly."
        )
        sys.exit(1)

def read_api_keys(google_key_file: str, openai_key_file: str) -> Dict[str, str]:
    """
    Read Google/OpenAI API keys from files, ensuring permissions are correct.
    Exit if there's any error.
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
    Append a single JSON line with 'role' and 'content' to 'filename'.
    """
    record = {"role": role, "content": content}
    try:
        with open(filename, "a", encoding="utf-8") as f:
            f.write(json.dumps(record))
            f.write("\n")
    except Exception as e:
        print(f"Could not save chat line: {e}")

def load_remembered_locations(filename: str) -> Dict[str, str]:
    """
    Load remembered locations from a JSON file.
    If the file doesn't exist or is invalid, return an empty dict.
    """
    if not os.path.exists(filename):
        return {}
    try:
        with open(filename, "r", encoding="utf-8") as f:
            data = json.load(f)
            if not isinstance(data, dict):
                return {}
            return {k: v for k, v in data.items()}
    except Exception:
        return {}

def save_remembered_locations(filename: str, locations: Dict[str, str]) -> None:
    """
    Write the entire dictionary of remembered locations to filename (JSON).
    """
    try:
        with open(filename, "w", encoding="utf-8") as f:
            json.dump(locations, f)
    except Exception as e:
        print(f"Failed to save remembered locations: {e}")

def tool_chat_completion(openai_api_key: str, messages: List[Dict[str, str]], verbose: bool) -> Dict[str, Any]:
    """
    tool_chat_completion: Calls OpenAI Chat Completions with the given messages.
    Returns {'content': str} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not openai_api_key:
        out["error"] = "Missing openai_api_key"
        return out
    url = "https://api.openai.com/v1/chat/completions"
    headers = {"Authorization": f"Bearer {openai_api_key}", "Content-Type": "application/json"}
    payload = {
        "model": "gpt-4o",
        "messages": messages,
        "temperature": 0.0,
    }
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
    if not choices:
        out["error"] = "No completion choices returned."
        return out
    out["content"] = choices[0].get("message", {}).get("content", "")
    return out

def tool_geocode_address(google_api_key: str, address: str, verbose: bool) -> Dict[str, Any]:
    """
    tool_geocode_address: Get latitude/longitude from an address using Geocoding API.
    Returns {'lat': float, 'lng': float} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not google_api_key:
        out["error"] = "Missing google_api_key"
        return out
    if not address:
        out["error"] = "Missing address"
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
    if not results:
        out["error"] = f"No geocoding results for '{address}'"
        return out
    location = results[0].get("geometry", {}).get("location", {})
    out["lat"] = location.get("lat", 0.0)
    out["lng"] = location.get("lng", 0.0)
    return out

def tool_text_search(google_api_key: str, query: str, verbose: bool) -> Dict[str, Any]:
    """
    tool_text_search: Use Places Text Search to find places matching the query.
    Returns {'results': [...]} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not google_api_key:
        out["error"] = "Missing google_api_key"
        return out
    if not query:
        out["error"] = "Missing query"
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

def tool_nearby_search(
    google_api_key: str,
    latlng: str,
    place_type: str,
    radius: int,
    verbose: bool
) -> Dict[str, Any]:
    """
    tool_nearby_search: Use Places Nearby Search with latlng, place_type, radius.
    Returns {'results': [...]} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not google_api_key:
        out["error"] = "Missing google_api_key"
        return out
    if not latlng:
        out["error"] = "Missing latlng"
        return out
    if not place_type:
        out["error"] = "Missing place_type"
        return out
    url = "https://maps.googleapis.com/maps/api/place/nearbysearch/json"
    params = {
        "key": google_api_key,
        "location": latlng,
        "type": place_type,
        "radius": radius,
    }
    if verbose:
        print(f"NearbySearch GET: {url} {params}")
    try:
        resp = requests.get(url, params=params)
    except Exception as e:
        out["error"] = f"Nearby Search request failed: {e}"
        return out
    if resp.status_code != 200:
        out["error"] = f"Nearby Search error {resp.status_code}, text: {resp.text}"
        return out
    data = resp.json()
    out["results"] = data.get("results", [])
    return out

def tool_get_distance_matrix(
    google_api_key: str,
    origin: str,
    destination: str,
    verbose: bool
) -> Dict[str, Any]:
    """
    Helper for route planning. 
    Returns {'distance_text', 'duration_text', ...} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not google_api_key:
        out["error"] = "Missing google_api_key"
        return out
    if not origin or not destination:
        out["error"] = "Missing origin or destination"
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
    if not rows:
        out["error"] = "No rows in Distance Matrix response"
        return out
    elements = rows[0].get("elements", [])
    if not elements:
        out["error"] = "No elements in Distance Matrix row"
        return out
    dist = elements[0].get("distance", {})
    dur = elements[0].get("duration", {})
    out["distance_text"] = dist.get("text", "")
    out["distance_value"] = dist.get("value", 0)
    out["duration_text"] = dur.get("text", "")
    out["duration_value"] = dur.get("value", 0)
    return out

def tool_plan_route(
    google_api_key: str,
    origin: str,
    destination: str,
    verbose: bool
) -> Dict[str, Any]:
    """
    tool_plan_route: Return distance/duration from origin to destination using Distance Matrix.
    Returns {'distance_text': str, 'duration_text': str} or {'error': str}.
    """
    dm_res = tool_get_distance_matrix(google_api_key, origin, destination, verbose)
    if "error" in dm_res:
        return {"error": dm_res["error"]}
    return {
        "distance_text": dm_res["distance_text"],
        "duration_text": dm_res["duration_text"],
    }

def tool_remember(file_path: str, label: str, latlng: str) -> Dict[str, Any]:
    """
    tool_remember: Save a label => lat,lng mapping to a JSON file. 
    Returns {'success': True} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    if not label:
        out["error"] = "Missing label"
        return out
    if not latlng:
        out["error"] = "Missing latlng"
        return out
    data = load_remembered_locations(file_path)
    data[label] = latlng
    save_remembered_locations(file_path, data)
    out["success"] = True
    return out

def tool_recall(file_path: str) -> Dict[str, Any]:
    """
    tool_recall: Load and return all remembered locations from the JSON file.
    Returns {'locations': {label: latlng}} or {'error': str}.
    """
    out: Dict[str, Any] = {}
    try:
        data = load_remembered_locations(file_path)
        out["locations"] = data
    except Exception as e:
        out["error"] = f"Error loading remembered locations: {e}"
    return out

def tool_help() -> None:
    """
    tool_help: Print out usage instructions for the available tools.
    """
    print("Callable tool actions include:")
    print('  {"action":"help"}')
    print('  {"action":"text_search","query":"Seattle coffee shops"}')
    print('  {"action":"nearby_search","latlng":"47.6062,-122.3321","place_type":"restaurant","radius":1000}')
    print('  {"action":"geocode","address":"1600 Amphitheatre Parkway"}')
    print('  {"action":"plan_route","origin":"47.6062,-122.3321","destination":"47.6205,-122.3493"}')
    print('  {"action":"remember","label":"work","latlng":"47.6205,-122.3493"}')
    print('  {"action":"recall"}  # to list known labels & latlngs')
    print("You can also type exit or quit to leave.")

def execute_tool_action(
    action: str,
    parsed: Dict[str, Any],
    google_api_key: str,
    openai_api_key: str,
    remembered_file: str,
    history_file: str,
    verbose: bool
) -> None:
    """
    Execute the requested tool action with given parsed arguments.
    """
    if action == "help":
        tool_help()
        return
    if action == "text_search":
        query = parsed.get("query", "")
        result = tool_text_search(google_api_key, query, verbose)
        if "error" in result:
            print(f"Text Search error: {result['error']}")
            return
        items = result.get("results", [])
        if not items:
            print("No results found.")
            return
        limit = min(3, len(items))
        for i in range(limit):
            r = items[i]
            name = r.get("name", "?")
            addr = r.get("formatted_address", "?")
            print(f"{i+1}. {name} - {addr}")
        return
    if action == "nearby_search":
        latlng = parsed.get("latlng", "")
        place_type = parsed.get("place_type", "")
        radius = parsed.get("radius", 1000)
        result = tool_nearby_search(google_api_key, latlng, place_type, radius, verbose)
        if "error" in result:
            print(f"Nearby Search error: {result['error']}")
            return
        items = result.get("results", [])
        if not items:
            print("No results found.")
            return
        limit = min(3, len(items))
        for i in range(limit):
            r = items[i]
            name = r.get("name", "?")
            vicinity = r.get("vicinity", "?")
            print(f"{i+1}. {name} - {vicinity}")
        return
    if action == "geocode":
        address = parsed.get("address", "")
        gdata = tool_geocode_address(google_api_key, address, verbose)
        if "error" in gdata:
            print(f"Geocode error: {gdata['error']}")
            return
        lat = gdata.get("lat", 0.0)
        lng = gdata.get("lng", 0.0)
        print(f"Coordinates for '{address}': {lat},{lng}")
        return
    if action == "plan_route":
        origin = parsed.get("origin", "")
        destination = parsed.get("destination", "")
        rdata = tool_plan_route(google_api_key, origin, destination, verbose)
        if "error" in rdata:
            print(f"Route error: {rdata['error']}")
            return
        dist_txt = rdata["distance_text"]
        dur_txt = rdata["duration_text"]
        print(f"Distance from '{origin}' to '{destination}': {dist_txt} in {dur_txt}")
        return
    if action == "remember":
        label = parsed.get("label", "")
        latlng = parsed.get("latlng", "")
        res = tool_remember(remembered_file, label, latlng)
        if "error" in res:
            print(f"Remember error: {res['error']}")
            return
        print(f"Remembered '{label}' as '{latlng}'")
        return
    if action == "recall":
        result = tool_recall(remembered_file)
        if "error" in result:
            print(f"Recall error: {result['error']}")
            return
        stored = result.get("locations", {})
        if not stored:
            print("No remembered locations.")
            return
        print("Remembered locations:")
        for k, v in stored.items():
            print(f"  {k} -> {v}")
        return
    print("Unknown action. Raw output:")
    print(parsed)

def handle_chat_loop(
    google_api_key: str,
    openai_api_key: str,
    remembered_file: str,
    history_file: str,
    verbose: bool
) -> None:
    """
    Main conversation loop. 
    The system prompt enumerates all the tool actions.
    """
    conversation: List[Dict[str, str]] = []
    system_prompt = (
        "You are an assistant that responds to user queries with JSON for a relevant tool call. "
        "Available tools: text_search, nearby_search, geocode, plan_route, remember, recall, help. "
        'Example: {"action":"text_search","query":"coffee in Seattle"}. '
        'If input not recognized, respond with {"action":"help"}.'
    )
    conversation.append({"role": "system", "content": system_prompt})
    save_chat_line("system", system_prompt, history_file)

    while True:
        try:
            user_input = input("> ")
        except (EOFError, KeyboardInterrupt):
            print("\nExiting.")
            break
        user_input = user_input.strip()
        if user_input.lower() in ("exit", "quit"):
            print("Exiting.")
            break
        conversation.append({"role": "user", "content": user_input})
        save_chat_line("user", user_input, history_file)

        completion = tool_chat_completion(openai_api_key, conversation, verbose)
        if "error" in completion:
            print(f"OpenAI error: {completion['error']}")
            continue
        assistant_msg = completion.get("content", "")
        conversation.append({"role": "assistant", "content": assistant_msg})
        save_chat_line("assistant", assistant_msg, history_file)

        try:
            parsed = json.loads(assistant_msg)
        except Exception:
            print("Assistant returned non-JSON content. Raw output:")
            print(assistant_msg)
            continue

        action = parsed.get("action", "")
        execute_tool_action(
            action=action,
            parsed=parsed,
            google_api_key=google_api_key,
            openai_api_key=openai_api_key,
            remembered_file=remembered_file,
            history_file=history_file,
            verbose=verbose,
        )

def main() -> None:
    parser = argparse.ArgumentParser(description="Chat-based Google Maps & memory script.")
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Print extra debug info (API requests)."
    )
    parser.add_argument(
        "--history-file",
        default="conversation.log",
        help="Append conversation lines to this file."
    )
    parser.add_argument(
        "--remembered-file",
        default="remembered_locations.json",
        help="File to store remembered locations (JSON)."
    )
    args = parser.parse_args()

    google_file = os.environ.get("GCLOUD_API_KEY_FILE", "")
    openai_file = os.environ.get("OPENAI_API_KEY_FILE", "")
    if not google_file or not openai_file:
        print("Must set GCLOUD_API_KEY_FILE and OPENAI_API_KEY_FILE environment vars.")
        sys.exit(1)

    keys = read_api_keys(google_file, openai_file)
    gkey = keys["google_api_key"]
    okey = keys["openai_api_key"]

    handle_chat_loop(
        google_api_key=gkey,
        openai_api_key=okey,
        remembered_file=args.remembered_file,
        history_file=args.history_file,
        verbose=args.verbose,
    )

if __name__ == "__main__":
    main()
