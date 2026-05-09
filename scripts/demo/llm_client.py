#!/usr/bin/env python3
"""LLM Client for Sui-Nexus Demo Agents.

Supports OpenAI and Groq providers.
Set LLM_PROVIDER=openai or LLM_PROVIDER=groq via environment variable.
"""

import json
import os
from typing import Optional

# Try to import OpenAI - it's a soft dependency
try:
    from openai import OpenAI
    OPENAI_AVAILABLE = True
except ImportError:
    OPENAI_AVAILABLE = False


class LLMClient:
    """Unified LLM client supporting multiple providers."""

    def __init__(self, provider: Optional[str] = None):
        self.provider = provider or os.getenv("LLM_PROVIDER", "openai")
        self._client = None
        self._model = None
        self._init_client()

    def _init_client(self):
        if self.provider == "openai":
            if not OPENAI_AVAILABLE:
                raise ImportError(
                    "OpenAI package not installed. Run: pip install openai"
                )
            api_key = os.getenv("OPENAI_API_KEY")
            if not api_key:
                raise ValueError("OPENAI_API_KEY environment variable is required")
            self._client = OpenAI(api_key=api_key)
            self._model = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

        elif self.provider == "groq":
            if not OPENAI_AVAILABLE:
                raise ImportError(
                    "OpenAI package not installed. Run: pip install openai"
                )
            api_key = os.getenv("GROQ_API_KEY")
            if not api_key:
                raise ValueError("GROQ_API_KEY environment variable is required")
            self._client = OpenAI(
                api_key=api_key,
                base_url="https://api.groq.com/openai/v1"
            )
            self._model = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")

        else:
            raise ValueError(f"Unsupported LLM provider: {self.provider}")

    def analyze_market_news(self, news: str) -> dict:
        """Analyze market news and return trading recommendation.

        Args:
            news: Market news text to analyze

        Returns:
            dict with keys: sentiment, confidence, action, target_tokens, reason
        """
        system_prompt = """You are a crypto market analyst specializing in DeFi and Sui ecosystem.
Given market news, analyze the sentiment and provide a trading recommendation.

Return JSON with exactly these fields:
- sentiment: "bullish", "bearish", or "neutral"
- confidence: float between 0.0 and 1.0
- action: "buy", "sell", or "hold"
- target_tokens: list of token symbols (e.g., ["SUI", "USDT"])
- reason: brief explanation of your analysis

Be objective and consider:
- Security incidents (hacks, exploits) → bearish
- Partnership announcements → bullish
- Market trends and sentiment
- Token utility and adoption"""

        response = self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": f"Analyze this market news:\n\n{news}"}
            ],
            response_format={"type": "json_object"},
            temperature=0.3,
        )

        result = json.loads(response.choices[0].message.content)

        # Validate response structure
        required_keys = {"sentiment", "confidence", "action", "target_tokens", "reason"}
        if not all(k in result for k in required_keys):
            raise ValueError(f"LLM returned invalid structure: {result}")

        # Validate sentiment
        if result["sentiment"] not in ("bullish", "bearish", "neutral"):
            result["sentiment"] = "neutral"

        # Validate action
        if result["action"] not in ("buy", "sell", "hold"):
            result["action"] = "hold"

        # Validate confidence range
        result["confidence"] = max(0.0, min(1.0, float(result["confidence"])))

        return result

    def parse_defi_intent(self, text: str) -> dict:
        """Parse natural language DeFi intent into structured format.

        Args:
            text: Natural language text like "Swap 1000 USDC for SUI with 0.5% slippage"

        Returns:
            dict with keys: action, params (amount, token_in, token_out, slippage, dest_addr)
        """
        system_prompt = """You are a DeFi intent parser. Convert natural language into structured intent.

Examples:
- "Swap 1000 USDC for SUI with 0.5% slippage"
  -> {"action": "Swap", "params": {"amount": "1000", "token_in": "USDC", "token_out": "SUI", "slippage": "0.5"}}

- "Send 500 SUI to 0x1234567890abcdef"
  -> {"action": "Transfer", "params": {"amount": "500", "token_in": "SUI", "dest_addr": "0x1234567890abcdef"}}

- "Buy 200 SUI"
  -> {"action": "Swap", "params": {"amount": "200", "token_in": "USDT", "token_out": "SUI", "slippage": "0.5"}}

Return JSON only. Supported actions: Swap, Transfer.
Always include amount as a string of the number (e.g., "1000" not 1000)."""

        response = self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": text}
            ],
            response_format={"type": "json_object"},
            temperature=0.1,
        )

        result = json.loads(response.choices[0].message.content)

        if "action" not in result or "params" not in result:
            raise ValueError(f"LLM returned invalid structure: {result}")

        return result


def main():
    """Test the LLM client interactively."""
    import sys

    print("🤖 LLM Client Test")
    print(f"Provider: {os.getenv('LLM_PROVIDER', 'openai')}")
    print()

    try:
        client = LLMClient()
        print("✅ LLM client initialized successfully")
        print(f"Model: {client._model}")
        print()

        # Interactive test
        if len(sys.argv) > 1:
            # Command line argument mode
            news = " ".join(sys.argv[1:])
            print(f"📰 Analyzing: {news}")
        else:
            news = input("📰 Enter market news to analyze: ")

        result = client.analyze_market_news(news)
        print()
        print("📋 Analysis Result:")
        print(json.dumps(result, indent=2))

    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("\nTo install dependencies, run:")
        print("  pip install -r requirements.txt")
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
