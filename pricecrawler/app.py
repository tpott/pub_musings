# app.py
# FastAPI application for Facebook webhook server

import hashlib
import hmac
import secrets
import string
from contextlib import asynccontextmanager
from typing import Dict, Any, Optional

import asyncio
from fastapi import FastAPI, Request, Header, HTTPException, Query
from fastapi.responses import PlainTextResponse, JSONResponse, FileResponse
from fastapi.templating import Jinja2Templates
import uvicorn

from facebook_loop import polling_loop, run_once
from serve_config import getConfig
from session_storage import (
    get_payment_session,
    complete_payment_session,
    delete_user_data,
    mark_user_payment_completed,
)

# Initialize Jinja2 templates
templates = Jinja2Templates(directory=".")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup: launch polling task. Shutdown: cancel gracefully."""
    polling_task = asyncio.create_task(polling_loop())
    yield
    polling_task.cancel()
    try:
        await polling_task
    except asyncio.CancelledError:
        pass


app = FastAPI(lifespan=lifespan)


# Helper functions

def verify_facebook_signature(body: bytes, signature: str, app_secret: str) -> bool:
    """Verify Facebook webhook signature using HMAC SHA256."""
    if app_secret is None:
        return False

    expected_signature = hmac.new(
        app_secret.encode('utf-8'),
        body,
        hashlib.sha256
    ).hexdigest()

    if signature.startswith('sha256='):
        signature = signature[7:]

    return hmac.compare_digest(expected_signature, signature)


def generate_confirmation_code() -> str:
    """Generate a random 8-character confirmation code."""
    alphabet = string.ascii_uppercase + string.digits
    return ''.join(secrets.choice(alphabet) for _ in range(8))


# Routes

@app.get("/status")
async def status():
    """Health check endpoint."""
    return PlainTextResponse("1-AM-ALIVE")


@app.get("/app-validation")
async def app_validation(
    hub_mode: Optional[str] = Query(None, alias="hub.mode"),
    hub_challenge: Optional[str] = Query(None, alias="hub.challenge"),
    hub_verify_token: Optional[str] = Query(None, alias="hub.verify_token"),
):
    """Facebook webhook validation endpoint."""
    if hub_challenge is None:
        raise HTTPException(status_code=400, detail="Missing hub.challenge parameter")
    return PlainTextResponse(hub_challenge)


@app.get("/page-validation")
async def page_validation(
    hub_mode: Optional[str] = Query(None, alias="hub.mode"),
    hub_challenge: Optional[str] = Query(None, alias="hub.challenge"),
    hub_verify_token: Optional[str] = Query(None, alias="hub.verify_token"),
):
    """Facebook page webhook validation endpoint."""
    if hub_challenge is None:
        raise HTTPException(status_code=400, detail="Missing hub.challenge parameter")
    return PlainTextResponse(hub_challenge)


@app.get("/pay")
async def pay_page(request: Request, r: Optional[str] = Query(None)):
    """Payment page with template rendering."""
    config = getConfig()

    # Get session if session_id provided
    session = None
    status = None
    if r is not None:
        session = get_payment_session(r)
        if session is not None:
            status = session['status']

    # Render template
    try:
        return templates.TemplateResponse(
            "pay.html.tmpl",
            {
                "request": request,
                "YOUR_APP_ID": config['facebook_app_id'],
                "YOUR_REQUEST_ID": r if session is not None else 'null',
                "YOUR_STATUS": status if status is not None else 'null',
            }
        )
    except Exception as e:
        raise HTTPException(status_code=404, detail=f"Pay template file not found: {e}")


@app.get("/privacy-policy")
async def privacy_policy():
    """Serve privacy policy text file."""
    try:
        return FileResponse("privacy-policy.txt", media_type="text/plain")
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="Privacy policy file not found")


@app.get("/terms")
async def terms_of_service():
    """Serve terms of service text file."""
    try:
        return FileResponse("terms-of-service.txt", media_type="text/plain")
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="Terms of service file not found")


@app.get("/robots.txt")
async def robots_txt():
    """Serve robots.txt file."""
    try:
        return FileResponse("robots.txt", media_type="text/plain")
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="Robots file not found")


@app.get("/deleted")
async def deleted(id: Optional[str] = Query(None)):
    """Deletion confirmation page."""
    if id is None:
        raise HTTPException(status_code=400, detail="No confirmation code provided")

    # TODO: handle deletion if we ever add data retention in the future
    return PlainTextResponse(f"SUCCESS {id}")


@app.get("/trigger-callback")
async def trigger_callback():
    """Manually trigger polling loop."""
    # Trigger immediate polling instead of waiting for next 15s interval
    asyncio.create_task(run_once({}))
    return PlainTextResponse("Callback triggered")


@app.get("/barcode_scanner.webp")
async def barcode_scanner_image():
    """Serve barcode scanner image."""
    try:
        return FileResponse("barcode_scanner.webp", media_type="image/webp")
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="Barcode scanner image file not found")


@app.get("/og/price-checker")
async def og_price_checker(request: Request):
    """Serve OG price checker template."""
    config = getConfig()

    try:
        return templates.TemplateResponse(
            "product-price-checker.html.tmpl",
            {
                "request": request,
                "YOUR_APP_ID": config['facebook_app_id'],
            }
        )
    except Exception as e:
        raise HTTPException(status_code=404, detail=f"Product price checker file not found: {e}")


@app.post("/payment-callback")
async def payment_callback(request: Request):
    """Handle payment completion callback."""
    try:
        data = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON in request body")

    session_id = data.get('session_id')
    facebook_response = data.get('facebook_response')

    if not session_id or not facebook_response:
        raise HTTPException(status_code=400, detail="Missing session_id or facebook_response")

    # Get session to retrieve user_id before completing it
    session = get_payment_session(session_id)
    if not session:
        raise HTTPException(status_code=404, detail="Session not found")

    success = complete_payment_session(session_id, facebook_response)

    if not success:
        raise HTTPException(status_code=500, detail="Failed to complete payment session")

    # Mark user payment as completed
    user_id = session['user_id']
    mark_user_payment_completed(user_id)

    return JSONResponse({"status": "ok"})


@app.post("/delete-me")
async def delete_me(
    request: Request,
    x_hub_signature_256: Optional[str] = Header(None, alias="X-Hub-Signature-256")
):
    """Handle user data deletion request with Facebook signature verification."""
    if not x_hub_signature_256:
        raise HTTPException(status_code=401, detail="Missing signature")

    body = await request.body()
    config = getConfig()
    facebook_app_secret = config['facebook_app_secret']

    if not verify_facebook_signature(body, x_hub_signature_256, facebook_app_secret):
        raise HTTPException(status_code=401, detail="Invalid signature")

    try:
        data = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON in request body")

    user_id = data.get('user_id')

    if user_id is None or len(user_id) == 0:
        raise HTTPException(status_code=400, detail="Missing user_id field")

    # Delete all user data
    delete_user_data(user_id)

    confirmation_code = generate_confirmation_code()
    print(f'Received verified deletion request for user_id: {user_id}, confirmation_code: {confirmation_code}')

    # Get hostname from request
    hostname_and_maybe_port = request.headers.get('Host', 'localhost')

    response_data = {
        "url": f"https://{hostname_and_maybe_port}/deleted?id={confirmation_code}",
        "confirmation_code": confirmation_code
    }

    return JSONResponse(response_data)


if __name__ == "__main__":
    config = getConfig()
    uvicorn.run(
        "app:app",
        host="0.0.0.0",
        port=8443,
        ssl_keyfile=config.get('webhook_key_file'),
        ssl_certfile=config.get('webhook_cert_file'),
    )
