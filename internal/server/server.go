package server

import (
	"context"
	"net/http"

	"coderefinery/internal/config"
	"coderefinery/internal/indexer"
	"coderefinery/internal/search"

	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg      config.ServerConfig
	searcher *search.Searcher
	indexer  *indexer.Indexer
	router   *gin.Engine
	srv      *http.Server
}

func NewServer(cfg config.ServerConfig, searcher *search.Searcher, idx *indexer.Indexer) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		cfg:      cfg,
		searcher: searcher,
		indexer:  idx,
		router:   router,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	if s.cfg.EnableCORS {
		s.router.Use(corsMiddleware())
	}

	s.router.POST("/search", s.handleSearch)
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/stats", s.handleStats)


	// Einfache GUI für http://localhost:8080/
	s.router.GET("/", func(c *gin.Context) {
	    c.Header("Content-Type", "text/html")
	    c.String(200, `
	        <html>
	        <head><title>Refinery Debug</title></head>
	        <body style="font-family: sans-serif; max-width: 800px; margin: 2rem auto;">
	            <h1>Refinery Debug Search</h1>
	            <input id="q" type="text" placeholder="Query..." style="width: 100%; padding: 10px;">
	            <div id="res" style="margin-top: 1rem;"></div>
	            <script>
	                document.getElementById('q').onkeydown = async e => {
	                    if(e.key !== 'Enter') return;
	                    const res = await fetch('/search', {
	                        method: 'POST', body: JSON.stringify({query: e.target.value, limit: 10})
	                    });
	                    const json = await res.json();
	                    document.getElementById('res').innerHTML = json.results.map(r =>
	                        '<div style="border:1px solid #ddd; padding:10px; margin:10px 0;">' +
	                        '<b>' + r.Chunk.FilePath + '</b> (' + r.CombinedScore.toFixed(2) + ')<br>' +
	                        '<pre style="background:#f4f4f4; overflow-x:auto;">' + r.Chunk.Content + '</pre>' +
	                        '</div>'
	                    ).join('');
	                }
	            </script>
	        </body>
	        </html>
	    `)
	})
}

func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:         ":" + s.cfg.Port,
		Handler:      s.router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		MaxHeaderBytes: int(s.cfg.MaxRequestSize),
	}

	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
