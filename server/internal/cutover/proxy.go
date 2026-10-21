package cutover

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/lingqiao/server/internal/config"
)

// Router optionally forwards selected admin routes to the Java sidecar.
type Router struct {
	cfg   config.CutoverConfig
	proxy *httputil.ReverseProxy
}

// routeRule describes one cutover-eligible route.
type routeRule struct {
	module   string
	javaPath string
	write    bool
	method   string
	dual     bool
}

// New builds a cutover router. When cutover is disabled, Wrap returns the local handler.
func New(cfg config.CutoverConfig) (*Router, error) {
	r := &Router{cfg: cfg}
	if !cfg.Enabled {
		return r, nil
	}
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil {
		return nil, err
	}
	r.proxy = httputil.NewSingleHostReverseProxy(base)
	originalDirector := r.proxy.Director
	r.proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = base.Host
	}
	return r, nil
}

// Wrap returns a handler that proxies to Java when the module cutover flag is enabled.
func (r *Router) Wrap(module string, local http.HandlerFunc) http.HandlerFunc {
	if r == nil || r.proxy == nil || !r.cfg.Enabled || !r.moduleEnabled(module) {
		return local
	}
	return func(w http.ResponseWriter, req *http.Request) {
		rule, ok := lookupRoute(req.URL.Path)
		if !ok || rule.module != module {
			local(w, req)
			return
		}
		if !r.methodAllowed(module, req.Method, rule) {
			local(w, req)
			return
		}
		clone := req.Clone(req.Context())
		clone.URL.Path = rule.javaPath
		clone.URL.RawPath = rule.javaPath
		r.proxy.ServeHTTP(w, clone)
	}
}

func (r *Router) methodAllowed(module, method string, rule routeRule) bool {
	if method == http.MethodGet || method == http.MethodHead {
		if rule.write && !rule.dual {
			return false
		}
		return rule.method == "" || rule.method == http.MethodGet
	}
	expected := rule.method
	if expected == "" {
		expected = http.MethodPost
	}
	if method != expected {
		return false
	}
	if rule.write || rule.dual {
		return r.writeEnabled(module)
	}
	return true
}

func (r *Router) moduleEnabled(module string) bool {
	switch module {
	case "admin":
		return r.cfg.Admin
	case "exam":
		return r.cfg.Exam
	case "audit":
		return r.cfg.Audit
	case "scripts":
		return r.cfg.Scripts
	case "cards":
		return r.cfg.Cards
	case "sessions":
		return r.cfg.Sessions
	case "agents":
		return r.cfg.Agents
	case "releases":
		return r.cfg.Releases
	case "payloads":
		return r.cfg.Payloads
	case "chat":
		return r.cfg.Chat
	case "blacklist":
		return r.cfg.Blacklist
	case "announcement":
		return r.cfg.Announcement
	case "agent-portal":
		return r.cfg.AgentPortal
	default:
		return false
	}
}

func (r *Router) writeEnabled(module string) bool {
	switch module {
	case "admin":
		return r.cfg.AdminWrite
	case "cards":
		return r.cfg.CardsWrite
	case "blacklist":
		return r.cfg.BlacklistWrite
	case "exam":
		return r.cfg.ExamWrite
	case "sessions":
		return r.cfg.SessionsWrite
	case "agents":
		return r.cfg.AgentsWrite
	case "scripts":
		return r.cfg.ScriptsWrite
	case "announcement":
		return r.cfg.AnnouncementWrite
	case "chat":
		return r.cfg.ChatWrite
	case "releases":
		return r.cfg.ReleasesWrite
	case "payloads":
		return r.cfg.PayloadsWrite
	case "agent-portal":
		return r.cfg.AgentPortalWrite
	default:
		return false
	}
}

var staticRoutes = map[string]routeRule{
	"/admin/api/login":            {module: "admin", javaPath: "/java/admin/api/login", write: true},
	"/admin/api/password":         {module: "admin", javaPath: "/java/admin/api/password", write: true},
	"/admin/api/dashboard":        {module: "admin", javaPath: "/java/admin/api/dashboard"},
	"/admin/api/stats":            {module: "admin", javaPath: "/java/admin/api/stats"},
	"/admin/api/ops/overview":     {module: "admin", javaPath: "/java/admin/api/ops/overview"},
	"/admin/api/modules/overview": {module: "admin", javaPath: "/java/admin/api/modules/overview"},
	"/admin/api/admin-users":      {module: "admin", javaPath: "/java/admin/api/admin-users", dual: true},
	"/admin/api/admin-user/update": {
		module: "admin", javaPath: "/java/admin/api/admin-user/update", write: true,
	},
	"/admin/api/invites":             {module: "admin", javaPath: "/java/admin/api/invites"},
	"/admin/api/invite/create":       {module: "admin", javaPath: "/java/admin/api/invite/create", write: true},
	"/admin/api/invite/delete":       {module: "admin", javaPath: "/java/admin/api/invite/delete", write: true},
	"/admin/api/exam/records":        {module: "exam", javaPath: "/java/admin/api/exam/records"},
	"/admin/api/exam/stats":          {module: "exam", javaPath: "/java/admin/api/exam/stats"},
	"/admin/api/exam/export":         {module: "exam", javaPath: "/java/admin/api/exam/export"},
	"/admin/api/audit":               {module: "audit", javaPath: "/java/admin/api/audit"},
	"/admin/api/scripts":             {module: "scripts", javaPath: "/java/admin/api/scripts"},
	"/admin/api/script":              {module: "scripts", javaPath: "/java/admin/api/script", dual: true},
	"/admin/api/cards":               {module: "cards", javaPath: "/java/admin/api/cards"},
	"/admin/api/sessions":            {module: "sessions", javaPath: "/java/admin/api/sessions"},
	"/admin/api/agents":              {module: "agents", javaPath: "/java/admin/api/agents"},
	"/admin/api/releases":            {module: "releases", javaPath: "/java/admin/api/releases", dual: true},
	"/admin/api/payloads":            {module: "payloads", javaPath: "/java/admin/api/payloads"},
	"/admin/api/chat/messages":       {module: "chat", javaPath: "/java/admin/api/chat/messages"},
	"/admin/api/chat/delete":         {module: "chat", javaPath: "/java/admin/api/chat/delete", write: true},
	"/admin/api/chat/system":         {module: "chat", javaPath: "/java/admin/api/chat/system", write: true},
	"/admin/api/chat/mute":           {module: "chat", javaPath: "/java/admin/api/chat/mute", write: true},
	"/admin/api/payloads/upload-dll": {module: "payloads", javaPath: "/java/admin/api/payloads/upload-dll", write: true},
	"/admin/api/payloads/manage":     {module: "payloads", javaPath: "/java/admin/api/payloads/manage", write: true},
	"/admin/api/payload/upload":      {module: "payloads", javaPath: "/java/admin/api/payload/upload", write: true},
    "/admin/api/blacklist":           {module: "blacklist", javaPath: "/java/admin/api/blacklist", dual: true},
	"/admin/api/announcement":        {module: "announcement", javaPath: "/java/admin/api/announcement", dual: true},
	"/admin/api/card/generate":       {module: "cards", javaPath: "/java/admin/api/cards/generate", write: true},
	"/admin/api/card/batch-generate": {module: "cards", javaPath: "/java/admin/api/cards/batch-generate", write: true},
	"/admin/api/card/update":         {module: "cards", javaPath: "/java/admin/api/cards/update", write: true},
	"/admin/api/card/update-details": {module: "cards", javaPath: "/java/admin/api/cards/update-details", write: true},
	"/admin/api/cards/bulk":          {module: "cards", javaPath: "/java/admin/api/cards/bulk", write: true},
	"/admin/api/cards/export":        {module: "cards", javaPath: "/java/admin/api/cards/export"},
	"/admin/api/cards/import":        {module: "cards", javaPath: "/java/admin/api/cards/import", write: true},
	"/admin/api/control/authorizations": {
		module: "cards", javaPath: "/java/admin/api/control/authorizations",
	},
	"/admin/api/control/authorizations/bulk/impact": {
		module: "cards", javaPath: "/java/admin/api/control/authorizations/bulk/impact",
		method: http.MethodPost,
	},
	"/admin/api/control/authorizations/bulk/actions": {
		module: "cards", javaPath: "/java/admin/api/control/authorizations/bulk/actions",
		write: true,
	},
	"/api/v1/exam/report":        {module: "exam", javaPath: "/java/api/v1/exam/report", write: true},
	"/api/v1/exam/collect":       {module: "exam", javaPath: "/java/api/v1/exam/collect", write: true},
	"/api/v1/activate":           {module: "sessions", javaPath: "/java/api/v1/activate", write: true},
	"/api/v1/heartbeat":          {module: "sessions", javaPath: "/java/api/v1/heartbeat", write: true},
	"/api/v1/deactivate":         {module: "sessions", javaPath: "/java/api/v1/deactivate", write: true},
	"/api/v1/announcement":       {module: "announcement", javaPath: "/java/api/v1/announcement"},
	"/api/v1/chat/profile":       {module: "chat", javaPath: "/java/api/v1/chat/profile", write: true},
	"/api/v1/chat/messages":      {module: "chat", javaPath: "/java/api/v1/chat/messages", write: true},
	"/api/v1/chat/presence":      {module: "chat", javaPath: "/java/api/v1/chat/presence", write: true},
	"/api/v1/chat/react":         {module: "chat", javaPath: "/java/api/v1/chat/react", write: true},
	"/api/v1/chat/send":          {module: "chat", javaPath: "/java/api/v1/chat/send", write: true},
	"/api/v1/dll":                {module: "payloads", javaPath: "/java/api/v1/dll"},
	"/api/v1/script":             {module: "scripts", javaPath: "/java/api/v1/script"},
	"/api/v1/key-exchange":       {module: "payloads", javaPath: "/java/api/v1/key-exchange", method: http.MethodPost},
	"/api/v1/update/check":       {module: "releases", javaPath: "/java/api/v1/update/check", write: true},
	"/api/v1/update/events":      {module: "releases", javaPath: "/java/api/v1/update/events", write: true},
	"/admin/api/force-logout":    {module: "sessions", javaPath: "/java/admin/api/force-logout", write: true},
	"/admin/api/agent/update":    {module: "agents", javaPath: "/java/admin/api/agent/update", write: true},
	"/admin/api/agent/cards":     {module: "agents", javaPath: "/java/admin/api/agent/cards"},
	"/admin/api/machines":        {module: "agents", javaPath: "/java/admin/api/machines"},
	"/admin/api/machine/cards":   {module: "agents", javaPath: "/java/admin/api/machine/cards"},
	"/admin/api/scripts/create":  {module: "scripts", javaPath: "/java/admin/api/scripts/create", write: true},
	"/admin/api/scripts/publish": {module: "scripts", javaPath: "/java/admin/api/scripts/publish", write: true},
	"/api/login":                 {module: "agent-portal", javaPath: "/java/agent/api/login", write: true},
	"/api/register":              {module: "agent-portal", javaPath: "/java/agent/api/register", write: true},
	"/api/dashboard":             {module: "agent-portal", javaPath: "/java/agent/api/dashboard"},
	"/api/cards":                 {module: "agent-portal", javaPath: "/java/agent/api/cards"},
	"/api/card/generate":         {module: "agent-portal", javaPath: "/java/agent/api/card/generate", write: true},
	"/api/card/batch-generate":   {module: "agent-portal", javaPath: "/java/agent/api/card/batch-generate", write: true},
	"/api/info":                  {module: "agent-portal", javaPath: "/java/agent/api/info"},
	"/api/password":              {module: "agent-portal", javaPath: "/java/agent/api/password", write: true},
}

func lookupRoute(path string) (routeRule, bool) {
	if rule, ok := staticRoutes[path]; ok {
		return rule, true
	}
	if strings.HasPrefix(path, "/admin/api/control/authorizations/") {
		suffix := strings.TrimPrefix(path, "/admin/api/control/authorizations/")
		if code, ok := strings.CutSuffix(suffix, "/impact"); ok {
			if !validAuthorizationCode(code) {
				return routeRule{}, false
			}
			return routeRule{
				module:   "cards",
				javaPath: "/java/admin/api/control/authorizations/" + code + "/impact",
				method:   http.MethodPost,
			}, true
		}
		if code, ok := strings.CutSuffix(suffix, "/actions"); ok {
			if !validAuthorizationCode(code) {
				return routeRule{}, false
			}
			return routeRule{
				module:   "cards",
				javaPath: "/java/admin/api/control/authorizations/" + code + "/actions",
				write:    true,
			}, true
		}
		if !validAuthorizationCode(suffix) {
			return routeRule{}, false
		}
		return routeRule{
			module:   "cards",
			javaPath: "/java/admin/api/control/authorizations/" + suffix,
		}, true
	}
	if strings.HasPrefix(path, "/api/v1/update/package/") {
		id := strings.TrimPrefix(path, "/api/v1/update/package/")
		if !validReleaseSegment(id) {
			return routeRule{}, false
		}
		return routeRule{
			module:   "releases",
			javaPath: "/java/api/v1/update/package/" + id,
		}, true
	}
	if strings.HasPrefix(path, "/api/v1/payload/") {
		rest := strings.TrimPrefix(path, "/api/v1/payload/")
		if id, ok := strings.CutSuffix(rest, "/info"); ok {
			if !validReleaseSegment(id) {
				return routeRule{}, false
			}
			return routeRule{
				module:   "payloads",
				javaPath: "/java/api/v1/payload/" + id + "/info",
			}, true
		}
		parts := strings.Split(rest, "/chunk/")
		if len(parts) != 2 || !validReleaseSegment(parts[0]) ||
			!validDecimalSegment(parts[1]) {
			return routeRule{}, false
		}
		return routeRule{
			module: "payloads",
			javaPath: "/java/api/v1/payload/" + parts[0] +
				"/chunk/" + parts[1],
		}, true
	}
	if strings.HasPrefix(path, "/admin/api/releases/") {
		rest := strings.TrimPrefix(path, "/admin/api/releases/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || !validReleaseSegment(parts[0]) {
			return routeRule{}, false
		}
		rule := routeRule{
			module:   "releases",
			javaPath: "/java/admin/api/releases/" + parts[0] + "/" + parts[1],
		}
		switch parts[1] {
		case "events":
			return rule, true
		case "publish", "pause", "rollback", "packages":
			rule.write = true
			rule.method = http.MethodPost
			return rule, true
		default:
			return routeRule{}, false
		}
	}
	if strings.HasPrefix(path, "/admin/api/scripts/") {
		suffix := strings.TrimPrefix(path, "/admin/api/scripts/")
		if suffix == "" {
			return routeRule{}, false
		}
		if id, ok := strings.CutSuffix(suffix, "/activate"); ok {
			if !validScriptID(id) {
				return routeRule{}, false
			}
			return routeRule{
				module:   "scripts",
				javaPath: "/java/admin/api/scripts/" + id + "/activate",
				write:    true,
				method:   http.MethodPost,
			}, true
		}
		if id, ok := strings.CutSuffix(suffix, "/delete"); ok {
			if !validScriptID(id) {
				return routeRule{}, false
			}
			return routeRule{
				module:   "scripts",
				javaPath: "/java/admin/api/scripts/" + id + "/delete",
				write:    true,
				method:   http.MethodDelete,
			}, true
		}
		if !validScriptID(suffix) {
			return routeRule{}, false
		}
		return routeRule{module: "scripts", javaPath: "/java/admin/api/scripts/" + suffix}, true
	}
	return routeRule{}, false
}

func validReleaseSegment(id string) bool {
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func validAuthorizationCode(code string) bool {
	if code == "" || strings.Contains(code, "/") || strings.Contains(code, "..") {
		return false
	}
	for _, c := range code {
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func validScriptID(id string) bool {
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		return false
	}
	if id == "create" || id == "publish" {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func validDecimalSegment(value string) bool {
	if value == "CHUNK-INDEX" {
		return true
	}
	if value == "" {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// mapToJavaPath keeps backwards compatibility for read-route mapping checks.
func mapToJavaPath(path string) (string, bool) {
	rule, ok := lookupRoute(path)
	if !ok || rule.write || (rule.method != "" && rule.method != http.MethodGet) {
		return "", false
	}
	return rule.javaPath, true
}
