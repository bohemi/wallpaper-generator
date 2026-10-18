import os
import time
import re
import requests
from dotenv import load_dotenv
from playwright.sync_api import sync_playwright

load_dotenv()

COOKIE_VALUE = os.getenv("PINTEREST_SESSION_COOKIE", "")
SCRAPE_MODE = os.getenv("SCRAPE_MODE", "personal")
API_ENDPOINT = os.getenv("BACKEND_URL", "http://52.66.150.54:8080/api/v1/wallpaper/batch")

TOPIC_URL = "https://www.pinterest.com/ideas/motivational-quotes/942699298453/"
HOME_URL = "https://www.pinterest.com/"

def upgrade_to_high_res(url: str) -> str:
    return re.sub(r'/(236x|474x)/', '/originals/', url)

def run_scraper(scroll_count: int = 5):
    target_url = HOME_URL if SCRAPE_MODE == "personal" else TOPIC_URL
    print(f"🚀 Running scraper in [{SCRAPE_MODE.upper()}] mode...")
    print(f"🌐 Target: {target_url}")

    found_urls = set()

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
        )

        if SCRAPE_MODE == "personal" and COOKIE_VALUE:
            context.add_cookies([
                {
                    "name": "_pinterest_sess",
                    "value": COOKIE_VALUE,
                    "domain": ".pinterest.com",
                    "path": "/",
                    "httpOnly": True,
                    "secure": True,
                    "sameSite": "Lax",
                }
            ])
            print("🔑 Injected personal session cookie.")

        page = context.new_page()
        page.goto(target_url, wait_until="domcontentloaded")
        time.sleep(4)

        for i in range(scroll_count):
            print(f"📖 Scrolling batch {i + 1}/{scroll_count}...")
            img_elements = page.query_selector_all("img[src*='pinimg.com']")
            
            for img in img_elements:
                src = img.get_attribute("src")
                if not src or not ("236x" in src or "474x" in src or "736x" in src):
                    continue

                if re.search(r'/\d{2,3}x\d{2,3}/', src):
                    continue

                # Potrait ASPect ratio filter
                dimensions = img.evaluate("""
                    el => ({
                        width: el.naturalWidth || 0,
                        height: el.naturalHeight || 0
                    })
                """)
                w = dimensions.get("width", 0)
                h = dimensions.get("height", 0)

                # Skip if dimensions couldn't be read yet
                if w == 0 or h == 0:
                    continue

                # Phone aspect ratio check:
                # Must be strictly taller than it is wide.
                # A standard phone screen (9:16) has an aspect ratio around 0.56.
                # enforce (w / h) <= 0.70 to reject squares and horizontal images.
                aspect_ratio = w / h
                if aspect_ratio > 0.70:
                    continue
                # ----------------------------------------

                high_res = upgrade_to_high_res(src)
                found_urls.add(high_res)

            page.evaluate("window.scrollBy(0, document.body.scrollHeight)")
            time.sleep(2.5)

        browser.close()

    print(f"🎯 Extracted {len(found_urls)} unique wallpaper candidates!")
    return list(found_urls)

def push_to_backend(urls: list):
    if not urls:
        print("⚠️ No URLs found to send.")
        return

    payload = {
        "source": f"pinterest-{SCRAPE_MODE}",
        "caption": f"Pinterest {SCRAPE_MODE.capitalize()} Feed",
        "urls": urls
    }

    try:
        response = requests.post(API_ENDPOINT, json=payload, timeout=15)
        if response.status_code == 200:
            print(f"👍 AWS Backend response: {response.json()}")
        else:
            print(f"🤨 Failed to push. Status: {response.status_code}, Body: {response.text}")
    except Exception as e:
        print(f"❌ Error connecting to AWS backend: {e}")

# only run the scraper if this script is executed directly
if __name__ == "__main__":
    urls = run_scraper(scroll_count=4)
    push_to_backend(urls)