# CodeRefinery - Intelligent Code Search Engine

High-performance semantic code search system with GraphQL API, hybrid caching, and multi-language support optimized for modern development workflows.

## System Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        UI[GraphQL Playground/Client]
    end
    
    subgraph "API Gateway - Go/Gin"
        Router[Gin Router]
        Auth[JWT Auth Middleware]
        Metrics[Prometheus Metrics]
        Router --> Auth
        Auth --> GQL[GraphQL Handler]
    end
    
    subgraph "Core Services"
        GQL --> RepoSvc[Repository Service]
        GQL --> SearchSvc[Search Service]
        GQL --> AuthSvc[Auth Service]
        
        RepoSvc --> Indexer
        SearchSvc --> ChunkRepo[Chunk Repository]
        SearchSvc --> Cache
    end
    
    subgraph "Indexing Pipeline"
        Indexer --> Parser[Universal Parser]
        Parser --> GoParser[Go AST Parser]
        Parser --> UniversalAST[Universal AST]
        
        GoParser --> Embedder[Ollama Embedder]
        UniversalAST --> Embedder
        
        Embedder --> VectorDB[(PostgreSQL + pgvector)]
    end
    
    subgraph "Search Pipeline"
        ChunkRepo --> VectorDB
        Embedder --> |Query Embedding| ChunkRepo
        Cache --> Redis[(Redis L2)]
        Cache --> Memory[Memory L1]
    end
    
    subgraph "External Services"
        Embedder --> Ollama[Ollama API]
    end
    
    UI --> Router
    
    style UI fill:#e1f5ff
    style GQL fill:#fff4e6
    style VectorDB fill:#f3e5f5
    style Ollama fill:#e8f5e9
```

## Data Flow

```mermaid
sequenceDiagram
    participant Client
    participant GraphQL
    participant RepoService
    participant Indexer
    participant Parser
    participant Ollama
    participant PostgreSQL
    
    Note over Client,PostgreSQL: Repository Creation Flow
    Client->>GraphQL: createRepository(name, path)
    GraphQL->>RepoService: Create(ctx, name, path)
    RepoService->>RepoService: Validate path exists
    RepoService->>PostgreSQL: INSERT INTO projects
    RepoService->>Indexer: Index(ctx, repo) [async]
    
    Note over Indexer,PostgreSQL: Indexing Process
    loop For each file
        Indexer->>Parser: Parse(filePath, content)
        Parser->>Parser: Detect language
        alt Go language
            Parser->>Parser: Use Go AST Parser
        else Other languages
            Parser->>Parser: Use Universal AST Parser
        end
        Parser-->>Indexer: CodeChunks[]
        
        Indexer->>Ollama: EmbedBatch(chunks)
        Ollama-->>Indexer: Embeddings[]
        
        Indexer->>PostgreSQL: INSERT INTO code_chunks
    end
    
    Indexer->>PostgreSQL: UPDATE projects status=ready
    
    Note over Client,PostgreSQL: Search Flow
    Client->>GraphQL: search(query, limit)
    GraphQL->>SearchService: Search(ctx, request)
    
    SearchService->>Cache: Get(cacheKey)
    alt Cache Hit
        Cache-->>SearchService: Cached results
    else Cache Miss
        SearchService->>Ollama: Embed(query)
        Ollama-->>SearchService: Query embedding
        
        SearchService->>PostgreSQL: VectorSearch (cosine similarity)
        PostgreSQL-->>SearchService: ChunkSearchResults[]
        
        SearchService->>Cache: Set(cacheKey, results)
    end
    
    SearchService-->>GraphQL: SearchResults[]
    GraphQL-->>Client: JSON response
```

## Core Components

### 1. GraphQL API Layer

**Schema Overview:**

```graphql
type Repository {
  id: ID!
  name: String!
  path: String!
  status: String!
  lastIndexed: Time
  fileCount: Int!
  chunkCount: Int!
}

type SearchResult {
  filePath: String!
  startLine: Int!
  endLine: Int!
  content: String!
  score: Float!
  signature: String
}

type Query {
  repositories: [Repository!]!
  search(query: String!, limit: Int): [SearchResult!]!
  me: User
  llmInfo: LLMInfo!
}

type Mutation {
  createRepository(name: String!, path: String!): Repository!
  reindexRepository(id: ID!): Boolean!
  deleteRepository(id: ID!): Boolean!
  register(username: String!, password: String!): AuthPayload!
  login(username: String!, password: String!): AuthPayload!
  setEmbeddingModel(model: String!): Boolean!
}
```

**Key Resolvers:**
- `createRepository`: Validates path, creates DB entry, triggers async indexing
- `search`: Semantic search with caching and vector similarity
- `setEmbeddingModel`: Changes model, clears indices, triggers reindex

### 2. Universal AST Parser

Language-agnostic parser supporting 50+ languages through intelligent heuristics:

**Supported Language Families:**

| Family | Languages | Parsing Strategy |
|--------|-----------|------------------|
| C-Family | C, C++, Rust, Go, Java, C#, JavaScript, TypeScript, Kotlin, Swift, PHP | Block-based (`{}`) |
| Python | Python | Indentation-based |
| Ruby | Ruby, Crystal | Block-based (`begin/end`) |
| Lisp | Lisp, Scheme, Clojure | S-expressions (`()`) |
| ML | OCaml, F#, Haskell, Elm | Pattern-based (`let/in`) |
| Shell | Bash, Zsh, Fish | Function-based |
| Assembly | x86, ARM, MIPS | Label-based |
| SQL | PostgreSQL, MySQL | Procedure-based |
| Lua | Lua | Block-based (`function/end`) |

**Parser Selection Logic:**

```go
func GetParser(lang string) Parser {
    if lang == "go" {
        return &GoParser{}  // Native Go AST for best quality
    }
    return NewUniversalASTParser()  // Heuristic parser for all others
}
```

**Chunk Types Detected:**
- `function`: Standalone functions
- `method`: Class/struct methods
- `class`: Class definitions
- `struct`: Struct definitions
- `interface`: Interface definitions
- `generic`: Fallback for unrecognized code blocks

### 3. Hybrid Caching System

Two-tier caching for optimal performance:

**L1 Cache (Memory):**
- In-process cache using `patrickmn/go-cache`
- 5-minute default expiration
- Zero network latency
- Limited to server memory

**L2 Cache (Redis):**
- Distributed cache for multi-instance deployments
- 1-hour default TTL (configurable)
- Shared across server restarts
- Graceful degradation if unavailable

**Cache Key Generation:**

```go
func generateCacheKey(req SearchRequest) string {
    data := fmt.Sprintf("%s|%d|%f", req.Query, req.Limit, req.MinScore)
    hash := sha256.Sum256([]byte(data))
    return "search:" + hex.EncodeToString(hash[:])
}
```

**Lookup Flow:**
1. Check L1 (memory) - instant if hit
2. Check L2 (Redis) - promote to L1 if hit
3. Execute search - store in both L1 and L2

### 4. Semantic Search Engine

**Vector Search with pgvector:**

```sql
SELECT c.*, 
       1 - (c.embedding <=> $1) as cosine_similarity
FROM code_chunks c
WHERE 1 - (c.embedding <=> $1) >= $2
ORDER BY c.embedding <=> $1
LIMIT $3
```

**Distance Operator (`<=>`):** Cosine distance in pgvector
**Similarity Calculation:** `1 - distance = similarity` (0 to 1 scale)

**Search Parameters:**
- `query`: Natural language search query
- `limit`: Maximum results (default 10)
- `min_score`: Minimum similarity threshold (default 0.25)

**Result Ranking:**
Results are sorted by cosine similarity (descending), with scores normalized to 0-1 range.

### 5. Circuit Breaker Pattern

Protects against cascade failures when external services (Ollama) are unavailable:

**Configuration:**
- Max requests in half-open: 5
- Interval: 10 seconds
- Timeout: 30 seconds (production)
- Trip condition: 3+ requests and 60%+ failure rate

**States:**
- Closed: Normal operation
- Open: All requests fail fast
- Half-Open: Testing recovery

## Installation

### Prerequisites

**System Requirements:**
- Go 1.21 or higher
- PostgreSQL 14+ with pgvector extension
- Ollama running locally
- Redis (optional, for distributed caching)
- Minimum 4GB free RAM

**Database Setup:**

```bash
# Install PostgreSQL and pgvector
sudo apt-get install postgresql-14 postgresql-14-pgvector

# Create database
sudo -u postgres psql
CREATE DATABASE coderefinery;
CREATE USER refinery WITH PASSWORD 'secret';
GRANT ALL PRIVILEGES ON DATABASE coderefinery TO refinery;

# Enable pgvector extension
\c coderefinery
CREATE EXTENSION vector;
```

**Ollama Models:**

```bash
# Embedding model (274MB)
ollama pull nomic-embed-text

# Alternative faster model (335MB)
ollama pull mxbai-embed-large
```

### Build Steps

1. Clone and initialize:

```bash
git clone https://github.com/marw-dev/coderefinery.git
cd coderefinery
go mod download
```

2. Run database migrations:

```bash
# Migrations are auto-applied on first startup
# Or manually via psql:
psql -U refinery -d coderefinery -f migrations/001_initial_schema.sql
```

3. Create configuration:

```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your settings
```

4. Build:

```bash
# Standard build
go build -o refinery cmd/main.go

# Optimized build
go build -ldflags="-s -w" -o refinery cmd/main.go
```

## Configuration

### config.yaml Structure

```yaml
environment: dev  # dev, staging, production

server:
  port: "8080"
  read_timeout: 30s
  write_timeout: 30s
  max_request_size: 52428800  # 50MB
  enable_cors: true
  allowed_origins:
    - "*"

database:
  driver: postgres
  source: "postgres://refinery:secret@localhost:5432/coderefinery?sslmode=disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 15m

llm:
  service: ollama
  host: "http://localhost:11434"
  embedding_model: "nomic-embed-text"
  timeout: 60s

indexer:
  supported_extensions:
    go: go
    py: python
    rs: rust
    js: javascript
    ts: typescript
    java: java
    cpp: cpp
    c: c
  exclude_paths:
    - node_modules
    - vendor
    - .git
    - __pycache__
    - dist
    - build
  min_chunk_size: 50
  max_chunk_size: 2000
  batch_size: 10

auth:
  jwt_secret: "change-me-to-a-very-secure-secret-key"
  jwt_expiry: 24h

search:
  default_limit: 10
  max_limit: 50
  min_score: 0.5

cache:
  enabled: true
  redis_url: "redis://localhost:6379/0"
  ttl: 1h

observability:
  logging:
    level: info  # debug, info, warn, error
    format: console  # console, json
  
  metrics:
    enabled: true
    path: "/metrics"
  
  tracing:
    enabled: false
    provider: jaeger
    endpoint: "http://localhost:14268/api/traces"
    sampling_rate: 1.0
    service_name: coderefinery
```

### Environment Variables

Override configuration via environment variables:

```bash
export REFINERY_ENVIRONMENT=production
export REFINERY_DATABASE_SOURCE="postgres://user:pass@host:5432/db"
export REFINERY_LLM_HOST="http://ollama:11434"
export REFINERY_AUTH_JWT_SECRET="production-secret-key"
export REFINERY_CACHE_REDIS_URL="redis://redis:6379/0"
```

## Usage

### Starting the Server

```bash
# Development mode (console logging)
./refinery

# Production mode (JSON logging)
REFINERY_ENVIRONMENT=production ./refinery

# Custom configuration file
./refinery --config /path/to/config.yaml

# Custom port
./refinery --port 9000
```

**Expected Output:**

```
{"level":"info","time":"2025-01-24T10:30:00Z","message":"Starting CodeRefinery in dev mode"}
{"level":"info","driver":"postgres","source":"postgres://...","message":"Connecting to database"}
{"level":"info","enabled":false,"message":"Tracing initialized"}
{"level":"info","path":"/metrics","message":"Metrics endpoint enabled"}
{"level":"info","port":"8080","message":"Playground ready"}
```

### GraphQL API Endpoints

**Playground:**
- URL: `http://localhost:8080/playground`
- Interactive GraphQL explorer

**Query Endpoint:**
- URL: `http://localhost:8080/query`
- POST requests with GraphQL queries
- Requires JWT token for authenticated queries

**Health Check:**
```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "up",
  "env": "dev"
}
```

**Metrics:**
```bash
curl http://localhost:8080/metrics
```

### GraphQL Query Examples

**Create Repository:**

```graphql
mutation {
  createRepository(
    name: "my-backend"
    path: "/home/user/projects/backend"
  ) {
    id
    name
    status
    fileCount
    chunkCount
  }
}
```

**Search Code:**

```graphql
query {
  search(query: "database connection pool", limit: 5) {
    filePath
    startLine
    endLine
    content
    score
    signature
  }
}
```

**List Repositories:**

```graphql
query {
  repositories {
    id
    name
    path
    status
    lastIndexed
    fileCount
    chunkCount
  }
}
```

**Authentication:**

```graphql
mutation {
  register(username: "admin", password: "secure123") {
    token
    user {
      id
      username
      role
    }
  }
}

mutation {
  login(username: "admin", password: "secure123") {
    token
    user {
      id
      username
      role
    }
  }
}
```

**Current User:**

```graphql
query {
  me {
    id
    username
    role
  }
}
```

**LLM Information:**

```graphql
query {
  llmInfo {
    currentModel
    availableModels
  }
}
```

**Change Embedding Model:**

```graphql
mutation {
  setEmbeddingModel(model: "mxbai-embed-large")
}
```

Note: This will delete all existing indices and trigger reindexing of all repositories.

### Using JWT Authentication

1. Register or login to get token:

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { login(username: \"admin\", password: \"secure123\") { token } }"
  }'
```

2. Use token in subsequent requests:

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "query": "{ me { username role } }"
  }'
```

## Performance Optimization

### Database Optimization

**Create Indexes:**

```sql
-- Speed up vector searches
CREATE INDEX idx_chunks_embedding ON code_chunks 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);

-- File lookups
CREATE INDEX idx_files_project_path ON files(project_id, path);

-- Chunk filtering
CREATE INDEX idx_chunks_chunk_type ON code_chunks(chunk_type);
```

**Tune pgvector:**

```sql
-- Increase work_mem for better vector operations
ALTER SYSTEM SET work_mem = '256MB';

-- Increase shared_buffers for caching
ALTER SYSTEM SET shared_buffers = '2GB';

SELECT pg_reload_conf();
```

### Memory Optimization

**Reduce Chunk Size:**

```yaml
indexer:
  min_chunk_size: 30   # from 50
  max_chunk_size: 1500 # from 2000
```

**Adjust Batch Size:**

```yaml
indexer:
  batch_size: 5  # from 10 (less memory, slower)
  batch_size: 20 # from 10 (more memory, faster)
```

**Limit Cache Size:**

```yaml
cache:
  ttl: 30m  # from 1h (faster eviction)
```

### Search Optimization

**Increase Minimum Score:**

```yaml
search:
  min_score: 0.6  # from 0.5 (fewer results, higher quality)
```

**Reduce Default Limit:**

```yaml
search:
  default_limit: 5   # from 10
  max_limit: 25      # from 50
```

## Advanced Features

### Multi-Repository Management

Create and manage multiple codebases:

```graphql
mutation {
  frontend: createRepository(name: "frontend", path: "/projects/frontend") { id }
  backend: createRepository(name: "backend", path: "/projects/backend") { id }
  mobile: createRepository(name: "mobile", path: "/projects/mobile") { id }
}

query {
  repositories {
    name
    status
    fileCount
  }
}
```

### Custom Language Support

Add new language profile in `internal/adapters/indexer/parser/universal.go`:

```go
profiles["dart"] = LanguageProfile{
    BlockStart: []string{"{"},
    BlockEnd:   []string{"}"},
    FunctionPatterns: []*regexp.Regexp{
        regexp.MustCompile(`(?m)^\s*(?:Future|Stream)?\s*\w+\s+\w+\s*\(`),
    },
    ClassPatterns: []*regexp.Regexp{
        regexp.MustCompile(`(?m)^\s*class\s+\w+`),
    },
    LineComment: []string{"//"},
    BlockComment: []struct{ Start, End string }{{Start: "/*", End: "*/"}},
}
```

Update configuration:

```yaml
indexer:
  supported_extensions:
    dart: dart
```

### Observability

**Prometheus Metrics:**

Available at `http://localhost:8080/metrics`

Key metrics:
- `coderefinery_http_requests_total`: Total HTTP requests by method, path, status
- `coderefinery_http_request_duration_seconds`: Request latency histogram
- `coderefinery_search_operation_duration_seconds`: Search performance

**Distributed Tracing:**

Enable Jaeger tracing:

```yaml
observability:
  tracing:
    enabled: true
    provider: jaeger
    endpoint: "http://localhost:14268/api/traces"
    sampling_rate: 1.0
```

Run Jaeger:

```bash
docker run -d \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

Access UI: `http://localhost:16686`

## Troubleshooting

### Database Connection Issues

**Problem:** "Failed to open db"

**Solutions:**

1. Verify PostgreSQL is running:
```bash
sudo systemctl status postgresql
```

2. Test connection:
```bash
psql -U refinery -d coderefinery -h localhost
```

3. Check pgvector extension:
```sql
SELECT * FROM pg_extension WHERE extname = 'vector';
```

### Indexing Failures

**Problem:** "Indexed 0 chunks"

**Diagnosis:**

```bash
# Check for supported files
find /project/path -type f \( -name "*.go" -o -name "*.py" -o -name "*.rs" \)

# Check .gitignore patterns
cat /project/path/.gitignore
```

**Solutions:**

1. Add file extensions to config
2. Remove overly broad exclude patterns
3. Verify file permissions

### Embedding Errors

**Problem:** "Failed to generate embeddings"

**Diagnosis:**

```bash
# Check Ollama status
curl http://localhost:11434/api/tags

# Test embedding directly
curl http://localhost:11434/api/embeddings \
  -d '{"model":"nomic-embed-text","prompt":"test"}'
```

**Solutions:**

1. Pull model: `ollama pull nomic-embed-text`
2. Restart Ollama: `systemctl restart ollama`
3. Check Ollama logs: `journalctl -u ollama -f`

### Redis Connection Issues

**Problem:** "Redis not available"

**Behavior:** Server continues with memory-only caching

**Solutions:**

1. Install Redis: `sudo apt-get install redis-server`
2. Start Redis: `sudo systemctl start redis`
3. Verify connection: `redis-cli ping`

### GraphQL Errors

**Problem:** "invalid or expired token"

**Solution:** Obtain new token via login mutation

**Problem:** "no user in context"

**Solution:** Include Authorization header with Bearer token

## Best Practices

### Query Design

**Effective Queries:**
- "JWT token validation in authentication module"
- "database connection pooling implementation"
- "error handling in API handlers"

**Ineffective Queries:**
- "show me code" (too vague)
- "everything about users" (too broad)

### Repository Organization

**Optimal Structure:**
- Separate repositories for distinct services
- Exclude build artifacts and dependencies
- Include documentation and configuration files

**Example:**

```graphql
mutation {
  createRepository(name: "backend", path: "/projects/backend")
  createRepository(name: "frontend", path: "/projects/frontend")
  createRepository(name: "shared", path: "/projects/shared-lib")
}
```

### Security Considerations

**JWT Secret Management:**

```bash
# Generate secure secret
openssl rand -base64 32

# Set via environment variable
export REFINERY_AUTH_JWT_SECRET="$(openssl rand -base64 32)"
```

**Network Security:**

```bash
# Restrict to localhost
sudo ufw deny 8080
sudo ufw allow from 127.0.0.1 to any port 8080
```

**Production Deployment:**

Use reverse proxy with TLS:

```nginx
server {
    listen 443 ssl;
    server_name coderefinery.internal;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Contributing

### Development Setup

1. Fork repository
2. Create feature branch: `git checkout -b feature/new-parser`
3. Install development tools:

```bash
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/99designs/gqlgen@latest
```

### Code Style

Run before committing:

```bash
# Format code
gofmt -w .
goimports -w .

# Lint
golangci-lint run

# Test
go test ./...

# Regenerate GraphQL code (if schema changed)
go run github.com/99designs/gqlgen generate
```

### Adding Language Support

1. Add profile to `universal.go`
2. Test with sample code
3. Update configuration example
4. Submit PR with test files

## License

MIT License - See LICENSE file for details

## Support

- GitHub Issues: Report bugs and feature requests
- Documentation: This README and inline code comments
- Health endpoint: Monitor server status at `/health`
- Metrics endpoint: Performance monitoring at `/metrics`
