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
    Verify file permissions are 0o400 or 0o600. Exit otherwise.
    """
    try:
        mode = os.stat(path).st_mode
        perms = mode & 0o777
    except Exception as e:
        print(f"Cannot stat file: {path}. Error: {e}")
        sys.exit(1)
    if perms != 0o400 and perms != 0o600:
        msg = f"File '{path}' must have permissions 400 or 600, found {oct(perms)}"
        print(msg)
        sys.exit(1)

def read_api_keys(google_key_file: str, openai_key_file: str) -> Dict[str, str]:
    """
    Read Google and OpenAI keys from files. Check permissions. Exit if invalid.
    """
    check_file_permissions(google_key_file)
    check_file_permissions(openai_key_file)
    try:
        with open(google_key_file, "r", encoding="utf-8") as fg:
            google_key = fg.read().strip()
        with open(openai_key_file, "r", encoding="utf-8") as fo:
            openai_key = fo.read().strip()
    except Exception as e:
        print(f"Error reading key files: {e}")
        sys.exit(1)
    return {"google_api_key": google_key, "openai_api_key": openai_key}

def tool_chat_completion(
    openai_api_key: str,
    messages: List[Dict[str, str]],
    verbose: bool,
) -> Dict[str, Any]:
    """
    Call the OpenAI Chat Completions API with the given message history.
    Return a dict with either {"content": "..."} or {"error": "..."}.
    """
    result: Dict[str, Any] = {}
    if openai_api_key == "":
        result["error"] = "Missing openai_api_key"
        return result
    url = "https://api.openai.com/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {openai_api_key}",
        "Content-Type": "application/json",
    }
    payload = {
        "model": "gpt-3.5-turbo",
        "messages": messages,
        "temperature": 0.0,
    }
    if verbose:
        print(f"OpenAI POST: {url}")
    try:
        response = requests.post(url, headers=headers, json=payload)
    except Exception as e:
        result["error"] = f"OpenAI request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"OpenAI API error {response.status_code}: {text}"
        return result
    data = response.json()
    choices = data.get("choices", [])
    if len(choices) == 0:
        result["error"] = "No completion choices returned."
        return result
    content = choices[0].get("message", {}).get("content", "")
    result["content"] = content
    return result

def tool_geocode_address(
    google_api_key: str,
    address: str,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Call the Google Geocoding API to get lat/lng from an address.
    Return {'lat': float, 'lng': float} or {'error': str}.
    """
    result: Dict[str, Any] = {}
    if google_api_key == "" or address == "":
        result["error"] = "Missing google_api_key or address"
        return result
    url = "https://maps.googleapis.com/maps/api/geocode/json"
    params = {"key": google_api_key, "address": address}
    if verbose:
        print(f"Geocode GET: {url} {params}")
    try:
        response = requests.get(url, params=params)
    except Exception as e:
        result["error"] = f"Geocoding request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"Geocoding error {response.status_code}, text: {text}"
        return result
    data = response.json()
    status = data.get("status", "")
    if status != "OK":
        result["error"] = f"Geocoding API status: {status}, response text: {response.text}"
        return result
    results = data.get("results", [])
    if len(results) == 0:
        result["error"] = f"No geocoding results found. Address: {address}"
        return result
    location = results[0].get("geometry", {}).get("location", {})
    lat = location.get("lat", 0.0)
    lng = location.get("lng", 0.0)
    result["lat"] = lat
    result["lng"] = lng
    return result

def tool_nearby_search(
    google_api_key: str,
    latlng: str,
    place_type: str,
    radius: int,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Places Nearby Search.
    Return {"results": [...]} or {"error": "..."}.
    """
    result: Dict[str, Any] = {}
    if google_api_key == "" or latlng == "":
        result["error"] = "Missing google_api_key or latlng"
        return result
    url = "https://maps.googleapis.com/maps/api/place/nearbysearch/json"
    params = {
        "key": google_api_key,
        "location": latlng,
        "type": place_type,
        "radius": radius,
    }
    if verbose:
        print(f"Places GET: {url} {params}")
    try:
        response = requests.get(url, params=params)
    except Exception as e:
        result["error"] = f"Nearby Search request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"Nearby Search error {response.status_code}, text: {text}"
        return result
    data = response.json()
    result["results"] = data.get("results", [])
    return result

def tool_text_search(
    google_api_key: str,
    query: str,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Places Text Search.
    Return {"results": [...]} or {"error": "..."}.
    """
    result: Dict[str, Any] = {}
    if google_api_key == "" or query == "":
        result["error"] = "Missing google_api_key or query"
        return result
    url = "https://maps.googleapis.com/maps/api/place/textsearch/json"
    params = {"key": google_api_key, "query": query}
    if verbose:
        print(f"TextSearch GET: {url} {params}")
    try:
        response = requests.get(url, params=params)
    except Exception as e:
        result["error"] = f"Text Search request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"Text Search error {response.status_code}, text: {text}"
        return result
    data = response.json()
    result["results"] = data.get("results", [])
    return result

def tool_find_place(
    google_api_key: str,
    input_str: str,
    input_type: str,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Places Find Place API.
    Return {"candidates": [...]} or {"error": "..."}.
    """
    result: Dict[str, Any] = {}
    if google_api_key == "" or input_str == "" or input_type == "":
        result["error"] = "Missing google_api_key or input or input_type"
        return result
    url = "https://maps.googleapis.com/maps/api/place/findplacefromtext/json"
    params = {
        "key": google_api_key,
        "input": input_str,
        "inputtype": input_type,  # e.g. 'textquery', 'phonenumber'
        "fields": "formatted_address,name,geometry",
    }
    if verbose:
        print(f"FindPlace GET: {url} {params}")
    try:
        response = requests.get(url, params=params)
    except Exception as e:
        result["error"] = f"Find Place request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"Find Place error {response.status_code}, text: {text}"
        return result
    data = response.json()
    result["candidates"] = data.get("candidates", [])
    return result

def tool_get_distance_matrix(
    google_api_key: str,
    origin: str,
    destination: str,
    verbose: bool,
) -> Dict[str, Any]:
    """
    Google Distance Matrix call. Return distance/duration or {"error": "..."}.
    """
    result: Dict[str, Any] = {}
    if google_api_key == "" or origin == "" or destination == "":
        result["error"] = "Missing google_api_key/origin/destination"
        return result
    url = "https://maps.googleapis.com/maps/api/distancematrix/json"
    params = {"key": google_api_key, "origins": origin, "destinations": destination}
    if verbose:
        print(f"DistanceMatrix GET: {url} {params}")
    try:
        response = requests.get(url, params=params)
    except Exception as e:
        result["error"] = f"Distance Matrix request failed: {e}"
        return result
    if response.status_code != 200:
        text = response.text
        result["error"] = f"Distance Matrix error {response.status_code}, text: {text}"
        return result
    data = response.json()
    rows = data.get("rows", [])
    if len(rows) < 1:
        result["error"] = "No rows in Distance Matrix response"
        return result
    elements = rows[0].get("elements", [])
    if len(elements) < 1:
        result["error"] = "No elements in Distance Matrix row"
        return result
    dist = elements[0].get("distance", {})
    dur = elements[0].get("duration", {})
    result["distance_text"] = dist.get("text", "")
    result["distance_value"] = dist.get("value", 0)
    result["duration_text"] = dur.get("text", "")
    result["duration_value"] = dur.get("value", 0)
    return result

def tool_help() -> None:
    """
    Print help info with example commands.
    """
    print("Commands you can try:")
    print("  search for coffee near seattle")
    print("  remember home is 40.7128,-74.0060")
    print("  plan route from home to seattle")
    print("  exit or quit")

def save_chat_line(role: str, content: str, filename: str) -> None:
    """
    Append a single JSON line with role/content to the history file.
    """
    record = {"role": role, "content": content}
    try:
        with open(filename, "a", encoding="utf-8") as f:
            f.write(json.dumps(record))
            f.write("\n")
    except Exception as e:
        print(f"Could not save chat line: {e}")

def handle_search(
    google_api_key: str,
    place_description: str,
    place_type: str,
    verbose: bool,
) -> None:
    """
    Try to interpret place_description as 'lat,lng' or geocode it.
    Then call nearby search for place_type within a fixed radius.
    Print up to 3 results.
    """
    lat_lng = ""
    parts = place_description.split(",")
    if len(parts) == 2:
        try:
            float(parts[0].strip())
            float(parts[1].strip())
            lat_lng = place_description.strip()
        except ValueError:
            lat_lng = ""
    if lat_lng == "":
        geo = tool_geocode_address(google_api_key, place_description, verbose)
        if "error" in geo:
            print(f"Geocoding error: {geo['error']}")
            return
        lat_lng = f"{geo['lat']},{geo['lng']}"
    radius = 5000
    resp = tool_nearby_search(google_api_key, lat_lng, place_type, radius, verbose)
    if "error" in resp:
        print(f"Search error: {resp['error']}")
        return
    results = resp.get("results", [])
    if len(results) == 0:
        print("No places found.")
        return
    limit = min(3, len(results))
    for i in range(limit):
        place = results[i]
        name = place.get("name", "?")
        vicinity = place.get("vicinity", "?")
        index = i + 1
        print(f"{index}. {name} - {vicinity}")

def handle_remember(
    known_locations: Dict[str, str],
    label: str,
    location: str,
) -> None:
    """
    Save a label: location mapping.
    """
    if label == "" or location == "":
        print("Invalid remember arguments.")
        return
    known_locations[label] = location
    print(f"Remembered '{label}' as '{location}'.")

def handle_plan_route(
    google_api_key: str,
    known_locations: Dict[str, str],
    from_label: str,
    to_label: str,
    verbose: bool,
) -> None:
    """
    Use distance matrix to plan route from known location A to B.
    """
    if from_label not in known_locations or to_label not in known_locations:
        print("Unknown label(s). Use 'remember' first.")
        return
    origin = known_locations[from_label]
    destination = known_locations[to_label]
    resp = tool_get_distance_matrix(google_api_key, origin, destination, verbose)
    if "error" in resp:
        print(f"Route error: {resp['error']}")
        return
    dist_txt = resp["distance_text"]
    dur_txt = resp["duration_text"]
    print(f"Route from '{from_label}' to '{to_label}': {dist_txt} in {dur_txt}")

def conversation_loop(
    google_api_key: str,
    openai_api_key: str,
    verbose: bool,
    history_file: str,
) -> None:
    known_locations: Dict[str, str] = {}
    conversation: List[Dict[str, str]] = []
    system_prompt = (
        "You are a helpful AI. You read the user's input and produce JSON for the "
        'relevant tool call, e.g. {"action":"search","location":"seattle","type":"coffee"}. '
        'If the user is not requesting a known action, respond with {"action":"help"}.'
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
        comp_out = tool_chat_completion(openai_api_key, conversation, verbose)
        if "error" in comp_out:
            print(f"OpenAI error: {comp_out['error']}")
            continue
        content = comp_out.get("content", "")
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
            f_lbl = parsed.get("from_label", "")
            t_lbl = parsed.get("to_label", "")
            handle_plan_route(google_api_key, known_locations, f_lbl, t_lbl, verbose)
            continue
        print("No recognized action. Raw output:")
        print(content)

def main() -> None:
    parser = argparse.ArgumentParser(description="Simple GMaps chat script.")
    parser.add_argument(
        "-v", "--verbose", action="store_true", help="Print verbose API requests."
    )
    parser.add_argument(
        "--history-file",
        default="conversation.log",
        help="Append conversation lines to this file.",
    )
    args = parser.parse_args()
    google_path = os.environ.get("GCLOUD_API_KEY_FILE", "")
    openai_path = os.environ.get("OPENAI_API_KEY_FILE", "")
    if google_path == "" or openai_path == "":
        print("Need GCLOUD_API_KEY_FILE and OPENAI_API_KEY_FILE in env vars.")
        sys.exit(1)
    keys = read_api_keys(google_path, openai_path)
    google_key = keys["google_api_key"]
    openai_key = keys["openai_api_key"]
    conversation_loop(
        google_api_key=google_key,
        openai_api_key=openai_key,
        verbose=args.verbose,
        history_file=args.history_file,
    )

if __name__ == "__main__":
    main()
