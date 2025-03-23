#!/usr/bin/env python3

# Ensure your API key files have restricted permissions (e.g. chmod 400 or 600).
# This script uses the Places API (Nearby Search) and the Distance Matrix API.
# Environment variables:
#   OPENAI_API_KEY_FILE: file holding the OpenAI API key
#   GCLOUD_API_KEY_FILE: file holding the Google Cloud (Maps) API key

import os
import sys
import requests
from typing import Any, Dict, List


def tool_read_api_keys(args: Dict[str, Any]) -> Dict[str, str]:
    f_g = os.environ.get("GCLOUD_API_KEY_FILE", "")
    f_o = os.environ.get("OPENAI_API_KEY_FILE", "")
    if f_g == "" or f_o == "":
        print("Please set GCLOUD_API_KEY_FILE and OPENAI_API_KEY_FILE.")
        sys.exit(1)

    try:
        with open(f_g, "r") as fg:
            gkey = fg.read().strip()
        with open(f_o, "r") as fo:
            okey = fo.read().strip()
    except Exception as e:
        print("Error reading key files:", e)
        sys.exit(1)

    return {"google_api_key": gkey, "openai_api_key": okey}


def tool_nearby_search(args: Dict[str, Any]) -> Dict[str, Any]:
    k = args.get("google_api_key", "")
    loc = args.get("location", "")
    t = args.get("type", "restaurant")
    r = args.get("radius", 5000)

    if k == "" or loc == "":
        err: Dict[str, Any] = {}
        err["error"] = "Missing google_api_key or location"
        return err

    url = "https://maps.googleapis.com/maps/api/place/nearbysearch/json"
    params = {
        "key": k,
        "location": loc,
        "radius": r,
        "type": t,
    }
    resp = requests.get(url, params=params)

    if resp.status_code != 200:
        err2: Dict[str, Any] = {}
        err2["error"] = f"{resp.status_code} {resp.text}"
        return err2

    data = resp.json()
    res: Dict[str, Any] = {}
    res["results"] = data.get("results", [])
    return res


def tool_get_distance_matrix(args: Dict[str, Any]) -> Dict[str, Any]:
    k = args.get("google_api_key", "")
    o = args.get("origin", "")
    d = args.get("destination", "")

    if k == "" or o == "" or d == "":
        err: Dict[str, Any] = {}
        err["error"] = "Missing google_api_key/origin/destination"
        return err

    url = "https://maps.googleapis.com/maps/api/distancematrix/json"
    params = {
        "key": k,
        "origins": o,
        "destinations": d,
    }
    resp = requests.get(url, params=params)

    if resp.status_code != 200:
        err2: Dict[str, Any] = {}
        err2["error"] = f"{resp.status_code} {resp.text}"
        return err2

    data = resp.json()
    rows = data.get("rows", [])
    if len(rows) == 0:
        err3: Dict[str, Any] = {}
        err3["error"] = "No rows in response"
        return err3

    elements = rows[0].get("elements", [])
    if len(elements) == 0:
        err4: Dict[str, Any] = {}
        err4["error"] = "No elements in first row"
        return err4

    e = elements[0]
    dist = e.get("distance", {})
    dur = e.get("duration", {})

    out: Dict[str, Any] = {}
    out["distance_text"] = dist.get("text", "")
    out["distance_value"] = dist.get("value", 0)
    out["duration_text"] = dur.get("text", "")
    out["duration_value"] = dur.get("value", 0)
    return out


def cmd_search(cmd: str, gkey: str, known: Dict[str, str]) -> None:
    parts = cmd.split(None, 3)
    if len(parts) < 4:
        print("Usage: search [type] near [location]")
        return

    ptype = parts[1]
    loc = parts[3]

    out = tool_nearby_search(
        {
            "google_api_key": gkey,
            "location": loc,
            "type": ptype,
        }
    )
    if "error" in out:
        print("Error:", out["error"])
        return

    results = out["results"]
    if len(results) == 0:
        print("No results.")
        return

    for i, place in enumerate(results[:3], 1):
        name = place.get("name", "?")
        vicinity = place.get("vicinity", "?")
        print(f"{i}. {name} - {vicinity}")


def cmd_remember(cmd: str, known: Dict[str, str]) -> None:
    parts = cmd.split(None, 3)
    if len(parts) < 4:
        print("Usage: remember [label] is [location]")
        return

    label = parts[1]
    loc = parts[3]
    known[label] = loc
    print(f"Remembered {label} = {loc}")


def cmd_plan_route(cmd: str, gkey: str, known: Dict[str, str]) -> None:
    parts = cmd.split()
    if len(parts) < 6:
        print("Usage: plan route from [label] to [label]")
        return

    frm = parts[3]
    to = parts[5]
    if frm not in known or to not in known:
        print("Unknown label(s). Use 'remember' first.")
        return

    out = tool_get_distance_matrix(
        {
            "google_api_key": gkey,
            "origin": known[frm],
            "destination": known[to],
        }
    )
    if "error" in out:
        print("Error:", out["error"])
        return

    dt = out["distance_text"]
    du = out["duration_text"]
    print(f"{frm} -> {to}: {dt} in {du}")


def main() -> None:
    keys = tool_read_api_keys({})
    gkey = keys["google_api_key"]
    known: Dict[str, str] = {}

    print("Commands:")
    print("  search [type] near [location]")
    print("  remember [label] is [location]")
    print("  plan route from [label] to [label]")
    print("  exit or quit")

    while True:
        cmd = input("> ").strip()
        if cmd.lower() == "exit" or cmd.lower() == "quit":
            break

        if cmd.startswith("search "):
            cmd_search(cmd, gkey, known)
        elif cmd.startswith("remember "):
            cmd_remember(cmd, known)
        elif cmd.startswith("plan route from "):
            cmd_plan_route(cmd, gkey, known)
        else:
            print("Unrecognized command.")


if __name__ == "__main__":
    main()
