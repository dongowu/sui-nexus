#!/usr/bin/env python3
"""Natural Language Intent Parser Service.

A simple HTTP server that parses natural language DeFi intents into structured format.
Run: python intent_parser.py

Endpoints:
- GET /health - Health check
- POST /parse - Parse natural language intent
"""

import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Optional

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# Load .env if exists
try:
    from dotenv import load_dotenv
    load_dotenv()
except ImportError:
    pass

# Import LLM client
try:
    from demo.llm_client import LLMClient
    LLM_AVAILABLE = True
except ImportError:
    LLM_AVAILABLE = False
    print("⚠️ Warning: LLM client not available, using rule-based parsing")


class IntentParser:
    """Parses natural language DeFi intents into structured format."""

    # Rule-based patterns for common DeFi intents
    PATTERNS = {
        "swap": [
            r"swap\s+(\d+(?:\.\d+)?)\s+(\w+)\s+for\s+(\w+)",
            r"exchange\s+(\d+(?:\.\d+)?)\s+(\w+)\s+to\s+(\w+)",
            r"buy\s+(\d+(?:\.\d+)?)\s+(\w+)",
        ],
        "transfer": [
            r"send\s+(\d+(?:\.\d+)?)\s+(\w+)\s+to\s+(0x[a-fA-F0-9]+)",
            r"transfer\s+(\d+(?:\.\d+)?)\s+(\w+)\s+to\s+(0x[a-fA-F0-9]+)",
        ],
    }

    def __init__(self, llm_client: Optional[LLMClient] = None):
        self.llm = llm_client

    def parse(self, text: str) -> dict:
        """Parse natural language text into structured intent.

        Args:
            text: Natural language intent text

        Returns:
            dict with action and params
        """
        text = text.strip().lower()

        # Try LLM first if available
        if self.llm is not None:
            try:
                return self.llm.parse_defi_intent(text)
            except Exception as e:
                print(f"⚠️ LLM parsing failed, falling back to rules: {e}")

        # Fallback to rule-based parsing
        return self._rule_based_parse(text)

    def _rule_based_parse(self, text: str) -> dict:
        """Simple rule-based parsing as fallback."""
        import re

        # Check for swap patterns
        for pattern in self.PATTERNS["swap"]:
            match = re.search(pattern, text)
            if match:
                groups = match.groups()
                if len(groups) == 3:
                    amount, token_in, token_out = groups
                else:
                    # "buy X token" pattern - assume USDT -> token
                    amount, token_out = groups
                    token_in = "USDT"

                return {
                    "action": "Swap",
                    "params": {
                        "amount": amount,
                        "token_in": token_in.upper(),
                        "token_out": token_out.upper(),
                        "slippage": "0.5"
                    }
                }

        # Check for transfer patterns
        for pattern in self.PATTERNS["transfer"]:
            match = re.search(pattern, text)
            if match:
                amount, token, dest_addr = match.groups()
                return {
                    "action": "Transfer",
                    "params": {
                        "amount": amount,
                        "token_in": token.upper(),
                        "dest_addr": dest_addr
                    }
                }

        # Default fallback
        return {
            "action": "Swap",
            "params": {
                "amount": "1000",
                "token_in": "USDT",
                "token_out": "SUI",
                "slippage": "0.5"
            },
            "error": "Could not parse intent precisely, using defaults"
        }


class Handler(BaseHTTPRequestHandler):
    """HTTP request handler for intent parsing."""

    parser: IntentParser = None

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            response = {"status": "healthy", "llm_available": LLM_AVAILABLE}
            self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"Intent Parser Service Running\n")
            self.wfile.write(f"LLM Available: {LLM_AVAILABLE}\n".encode())

    def do_POST(self):
        if self.path != "/parse":
            self.send_error(404, "Not Found")
            return

        content_length = int(self.headers.get("Content-Length", 0))
        if content_length == 0:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(json.dumps({"error": "missing body"}).encode())
            return

        body = self.rfile.read(content_length).decode()

        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(json.dumps({"error": "invalid JSON"}).encode())
            return

        text = data.get("text", "")
        if not text:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(json.dumps({"error": "missing text field"}).encode())
            return

        try:
            result = self.parser.parse(text)
            self.send_response(200)
            self.end_headers()
            self.wfile.write(json.dumps(result).encode())
        except Exception as e:
            self.send_response(500)
            self.end_headers()
            self.wfile.write(json.dumps({"error": str(e)}).encode())

    def log_message(self, format, *args):
        """Override to reduce log verbosity."""
        if self.path != "/health":
            print(f"[{self.address_string()}] {format % args}")


def main():
    """Start the intent parser service."""
    port = int(os.getenv("NLP_PORT", "8081"))

    # Initialize LLM client if available
    llm_client = None
    if LLM_AVAILABLE:
        try:
            provider = os.getenv("LLM_PROVIDER", "openai")
            llm_client = LLMClient(provider=provider)
            # Test the client
            llm_client.analyze_market_news("test")
            print(f"✅ LLM client initialized ({provider})")
        except Exception as e:
            print(f"⚠️ LLM client init failed: {e}")
            llm_client = None

    Handler.parser = IntentParser(llm_client)

    server = HTTPServer(("0.0.0.0", port), Handler)
    print(f"🤖 Intent Parser Service listening on port {port}")
    print(f"   LLM: {'enabled' if llm_client else 'disabled (rule-based fallback)'}")
    print(f"   Endpoints:")
    print(f"   - GET  /health - Health check")
    print(f"   - POST /parse  - Parse intent (body: {{'text': '...'}})")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n👋 Shutting down...")
        server.shutdown()


if __name__ == "__main__":
    main()
