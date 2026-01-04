"""
title: CodeRefinery - Intelligent Code Search
author: open-webui
author_url: https://github.com/open-webui
funding_url: https://github.com/open-webui
version: 2.0.0
license: MIT
requirements: requests
"""

from enum import Enum
from typing import Any, Callable, Optional

import requests


class FilterPreset(Enum):
    """Common filter presets for different search scenarios"""

    API = {"chunk_types": ["function", "class"], "path_filter": "api"}
    AUTH = {"path_filter": "auth"}
    DATABASE = {"path_filter": "db", "chunk_types": ["function"]}
    FRONTEND = {"languages": ["javascript", "typescript", "jsx", "tsx"]}
    BACKEND = {"languages": ["go", "python", "java"]}


class Tools:
    def __init__(self):
        self.valves = self.Valves()

    class Valves:
        def __init__(self):
            self.REFINERY_URL = "http://host.docker.internal:8080"
            self.DEFAULT_LIMIT = 5
            self.MAX_LIMIT = 15
            self.MIN_SCORE = 0.3  # Updated to match new default
            self.TIMEOUT_SECONDS = 15
            self.INCLUDE_SCORES = True
            self.VERBOSE_ERRORS = True

    def search_codebase(
        self,
        query: str,
        limit: Optional[int] = None,
        languages: Optional[list] = None,
        path_filter: Optional[str] = None,
        chunk_types: Optional[list] = None,
        min_score: Optional[float] = None,
        __user__: dict = None,
        __event_emitter__: Callable[[dict], Any] = None,
    ) -> str:
        """
        Search the codebase for relevant code snippets using hybrid semantic + keyword search.

        Use this tool when the user asks about:
        - Specific functions, classes, or interfaces
        - How something is implemented
        - Where certain logic exists
        - Code architecture or patterns
        - Finding examples or best practices
        - Debugging or understanding code flow

        Args:
            query: Natural language description of what to search for
            limit: Maximum number of results (default: 5, max: 15)
            languages: Filter by programming languages (e.g. ["go", "python", "rust"])
            path_filter: Filter by path substring (e.g. "internal/auth")
            chunk_types: Filter by code type (e.g. ["function", "class", "interface"])
            min_score: Minimum relevance score (0.0-1.0, default: 0.3)

        Examples:
            - search_codebase("user authentication", languages=["go"])
            - search_codebase("database connection pool", path_filter="internal/db")
            - search_codebase("JWT token validation", chunk_types=["function"])
            - search_codebase("error handling middleware", min_score=0.5)
        """

        self._emit_status(__event_emitter__, f"🔍 Searching: {query}", False)

        # Validate and normalize inputs
        limit = self._validate_limit(limit)
        min_score = min_score if min_score is not None else self.valves.MIN_SCORE

        try:
            # Health check with early return
            health_data = self._check_health()
            if isinstance(health_data, str):  # Error message
                return health_data

            total_chunks = health_data.get("chunks", 0)
            if total_chunks == 0:
                return self._format_error(
                    "CodeRefinery has indexed 0 code chunks.",
                    "The project might be empty or still indexing. Wait a moment and try again.",
                )

            # Build search payload with filters
            payload = self._build_search_payload(
                query, limit, languages, path_filter, chunk_types, min_score
            )

            # Execute search
            search_url = f"{self.valves.REFINERY_URL}/search"
            response = requests.post(
                search_url,
                json=payload,
                timeout=self.valves.TIMEOUT_SECONDS,
                headers={"Content-Type": "application/json"},
            )

            if response.status_code != 200:
                error_detail = ""
                try:
                    error_detail = response.json().get("error", "")
                except:
                    pass
                return self._format_error(
                    f"Search failed with HTTP {response.status_code}",
                    error_detail
                    or "The server encountered an error processing your request.",
                )

            data = response.json()
            results = data.get("results", [])
            took = data.get("took", "unknown")

            if not results:
                self._emit_status(__event_emitter__, "❌ No relevant code found", True)
                return self._format_no_results(query, total_chunks, payload)

            # Format and return results
            formatted = self._format_results(results, query, took, payload)
            self._emit_status(
                __event_emitter__, f"✅ Found {len(results)} snippets", True
            )

            return formatted

        except requests.exceptions.Timeout:
            return self._format_error(
                "Search request timed out",
                f"Query took longer than {self.valves.TIMEOUT_SECONDS}s. Try simplifying your search.",
            )
        except requests.exceptions.ConnectionError:
            return self._format_error(
                "Cannot connect to CodeRefinery server",
                "Start the server with: `./refinery serve [project-path]`",
            )
        except requests.exceptions.RequestException as e:
            return self._format_error("Network error", str(e))
        except Exception as e:
            return self._format_error("Unexpected error", str(e))

    def get_codebase_stats(
        self,
        __user__: dict = None,
        __event_emitter__: Callable[[dict], Any] = None,
    ) -> str:
        """
        Get detailed statistics about the indexed codebase.

        Returns information about:
        - Total indexed chunks and files
        - Language distribution
        - Last indexing time
        - Server status
        """

        try:
            stats_url = f"{self.valves.REFINERY_URL}/stats"
            response = requests.get(stats_url, timeout=5)

            if response.status_code == 200:
                data = response.json()

                # Format language statistics
                languages = data.get("Languages", {})
                lang_stats = "\n".join(
                    [
                        f"  - {lang}: {count} chunks"
                        for lang, count in sorted(
                            languages.items(), key=lambda x: x[1], reverse=True
                        )
                    ]
                )

                last_indexed = data.get("LastIndexed", "Unknown")

                return (
                    "**CodeRefinery Statistics**\n\n"
                    f"**Total Chunks:** {data.get('TotalChunks', 0)}\n"
                    f"**Total Files:** {data.get('TotalFiles', 0)}\n"
                    f"**Last Indexed:** {last_indexed}\n\n"
                    "**Language Distribution:**\n"
                    f"{lang_stats}\n\n"
                    "Server is operational and ready for queries"
                )
            else:
                return self._format_error(
                    "Stats endpoint unavailable", f"HTTP {response.status_code}"
                )

        except Exception as e:
            return self._format_error("Could not retrieve stats", str(e))

    def search_with_preset(
        self,
        query: str,
        preset: str,
        __user__: dict = None,
        __event_emitter__: Callable[[dict], Any] = None,
    ) -> str:
        """
        Search using predefined filter presets for common scenarios.

        Available presets:
        - "api": Search API endpoints and handlers
        - "auth": Search authentication/authorization code
        - "database": Search database queries and connections
        - "frontend": Search frontend code (JS/TS)
        - "backend": Search backend code (Go/Python/Java)

        Args:
            query: What to search for
            preset: One of the predefined presets above

        Example:
            search_with_preset("validate user token", "auth")
        """

        preset_filters = {
            "api": {"chunk_types": ["function", "class"], "path_filter": "api"},
            "auth": {"path_filter": "auth"},
            "database": {"path_filter": "db", "chunk_types": ["function"]},
            "frontend": {"languages": ["javascript", "typescript", "jsx", "tsx"]},
            "backend": {"languages": ["go", "python", "java"]},
        }

        filters = preset_filters.get(preset.lower())
        if not filters:
            return self._format_error(
                f"Unknown preset: {preset}",
                f"Available presets: {', '.join(preset_filters.keys())}",
            )

        return self.search_codebase(
            query=query,
            languages=filters.get("languages"),
            path_filter=filters.get("path_filter"),
            chunk_types=filters.get("chunk_types"),
            __user__=__user__,
            __event_emitter__=__event_emitter__,
        )

    # ==================== PRIVATE HELPER METHODS ====================

    def _check_health(self) -> dict | str:
        """Check server health and return status or error message"""
        try:
            health_url = f"{self.valves.REFINERY_URL}/health"
            response = requests.get(health_url, timeout=2)

            if response.status_code == 200:
                return response.json()
            else:
                return self._format_error(
                    "Server health check failed", f"HTTP {response.status_code}"
                )
        except requests.exceptions.ConnectionError:
            return self._format_error(
                "Cannot connect to CodeRefinery",
                "Ensure server is running: `./refinery serve [path]`",
            )
        except requests.exceptions.Timeout:
            return self._format_error(
                "Health check timed out", "Server may be overloaded"
            )

    def _validate_limit(self, limit: Optional[int]) -> int:
        """Validate and clamp the result limit"""
        if limit is None:
            return self.valves.DEFAULT_LIMIT
        return max(1, min(limit, self.valves.MAX_LIMIT))

    def _build_search_payload(
        self,
        query: str,
        limit: int,
        languages: Optional[list],
        path_filter: Optional[str],
        chunk_types: Optional[list],
        min_score: float,
    ) -> dict:
        """Build the search request payload"""
        payload = {
            "query": query,
            "limit": limit,
            "min_score": min_score,
        }

        if languages:
            payload["languages"] = languages
        if path_filter:
            payload["path_filter"] = path_filter
        if chunk_types:
            payload["chunk_types"] = chunk_types

        return payload

    def _format_results(
        self, results: list, query: str, took: str, payload: dict
    ) -> str:
        """Format search results for LLM consumption"""

        # Build filter info
        filters_applied = []
        if payload.get("languages"):
            filters_applied.append(f"Languages: {', '.join(payload['languages'])}")
        if payload.get("path_filter"):
            filters_applied.append(f"Path: *{payload['path_filter']}*")
        if payload.get("chunk_types"):
            filters_applied.append(f"Types: {', '.join(payload['chunk_types'])}")

        filter_info = " | ".join(filters_applied) if filters_applied else "No filters"

        output = [
            f"# Code Search Results: '{query}'",
            f"**Found:** {len(results)} snippets | **Time:** {took} | **Filters:** {filter_info}\n",
        ]

        for idx, result in enumerate(results, 1):
            # Handle both old and new response format
            chunk = result.get("chunk") or result.get("Chunk", {})

            # Extract metadata (handle both formats)
            file_path = chunk.get("file_path") or chunk.get("FilePath", "Unknown")
            start_line = chunk.get("start_line") or chunk.get("StartLine", "?")
            end_line = chunk.get("end_line") or chunk.get("EndLine", "?")
            language = chunk.get("language") or chunk.get("Language", "txt")
            chunk_type = chunk.get("chunk_type") or chunk.get("ChunkType", "generic")
            signature = chunk.get("signature") or chunk.get("Signature", "")
            content = chunk.get("content") or chunk.get("Content", "")

            # Scores (handle both formats)
            semantic = result.get("semantic_score") or result.get("SemanticScore", 0)
            keyword = result.get("keyword_score") or result.get("KeywordScore", 0)
            combined = result.get("combined_score") or result.get("CombinedScore", 0)

            # Format result
            output.append(f"## [{idx}] {file_path}")
            output.append(
                f"Lines {start_line}-{end_line} | "
                f"{language} | "
                f"{chunk_type} | "
                f"{combined:.1%} relevance"
            )

            if signature:
                output.append(f"**Signature:** `{signature}`")

            if self.valves.INCLUDE_SCORES:
                output.append(f"*Semantic: {semantic:.1%} | Keyword: {keyword:.1%}*")

            output.append(f"\n```{language}")
            output.append(content.strip())
            output.append("```\n")

            if idx < len(results):
                output.append("---\n")

        # Footer
        output.append(
            "\n**Tips:**\n"
            "- Higher relevance scores indicate better matches\n"
            "- Use filters (languages, path_filter, chunk_types) to narrow results\n"
            "- Ask follow-up questions about specific files or functions"
        )

        return "\n".join(output)

    def _format_no_results(self, query: str, total_chunks: int, payload: dict) -> str:
        """Format message when no results are found"""

        filters = []
        if payload.get("languages"):
            filters.append(f"languages: {', '.join(payload['languages'])}")
        if payload.get("path_filter"):
            filters.append(f"path containing '{payload['path_filter']}'")
        if payload.get("chunk_types"):
            filters.append(f"types: {', '.join(payload['chunk_types'])}")

        filter_text = f" with filters ({', '.join(filters)})" if filters else ""

        return (
            f"**❌ No Results Found**\n\n"
            f"Query: `{query}`{filter_text}\n\n"
            f"The codebase has **{total_chunks}** indexed chunks, but none matched your search.\n\n"
            "**Suggestions:**\n"
            "- Try different keywords or synonyms\n"
            "- Remove or adjust filters\n"
            "- Be more general in your query\n"
            "- Check if relevant files are in supported languages\n"
            f"- Lower min_score (currently {payload.get('min_score', 0.3)})"
        )

    def _format_error(self, title: str, detail: str = "") -> str:
        """Format error messages with troubleshooting"""
        message = f"**⚠️ {title}**\n\n"

        if detail:
            message += f"{detail}\n\n"

        if self.valves.VERBOSE_ERRORS:
            message += (
                "**Troubleshooting:**\n"
                f"1. Server running? `./refinery serve [path]`\n"
                f"2. Correct URL? `{self.valves.REFINERY_URL}`\n"
                "3. Check server logs for errors\n"
                "4. Verify project has supported code files\n"
                "5. Check if Ollama is running for embeddings"
            )

        return message

    def _emit_status(self, emitter: Optional[Callable], message: str, done: bool):
        """Emit status update if emitter is available"""
        if emitter:
            emitter(
                {
                    "type": "status",
                    "data": {
                        "description": message,
                        "done": done,
                    },
                }
            )
