package tools

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/norunners/openrig/internal/ids"
	"github.com/norunners/openrig/internal/invocation"
	"github.com/norunners/openrig/internal/project"
	"github.com/norunners/openrig/internal/result"
)

// maxCorrelationMetadataBytes bounds optional client labels before they reach
// logs, traces, hooks, or persisted attribution.
const maxCorrelationMetadataBytes = 256

// ResourceResolver supplies canonical lifecycle scopes without coupling MCP
// routing to concrete worktree, turn, or revision managers. Each method must
// resolve the exact requested resource or return an error.
type ResourceResolver interface {
	ResolveActiveTurn(context.Context, TurnID) (invocation.Scope, error)
	ResolveTurn(context.Context, TurnID) (invocation.Scope, error)
	ResolveWorktree(context.Context, WorktreeID) (invocation.Scope, error)
	ResolveRevision(context.Context, RevisionID) (invocation.Scope, error)
}

type RoutingOptions struct {
	Scope     Scope
	Projects  *project.Resolver
	Resources ResourceResolver
	RuntimeID string
	Transport string
}

// resolvedCall is the single decoded and routed value consumed by future
// handler middleware. Keeping arguments typed here prevents a second decode at
// the handler boundary.
type resolvedCall struct {
	Invocation invocation.Invocation
	Arguments  toolArguments
}

type scopedWorkArguments interface {
	toolArguments
	routingScope() scopeArguments
}

func (arguments shellArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments applyPatchArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processStartArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processStatusArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processReadArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processStopArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processRestartArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments processKillArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

func (arguments skillActivateArguments) routingScope() scopeArguments {
	return arguments.scopeArguments
}

var (
	_ scopedWorkArguments = shellArguments{}
	_ scopedWorkArguments = applyPatchArguments{}
	_ scopedWorkArguments = processStartArguments{}
	_ scopedWorkArguments = processStatusArguments{}
	_ scopedWorkArguments = processReadArguments{}
	_ scopedWorkArguments = processStopArguments{}
	_ scopedWorkArguments = processRestartArguments{}
	_ scopedWorkArguments = processKillArguments{}
	_ scopedWorkArguments = skillActivateArguments{}
)

func resolveCall(
	ctx context.Context,
	request mcp.CallToolRequest,
	options RoutingOptions,
) (resolvedCall, error) {
	call := resolvedCall{
		Invocation: invocation.Invocation{
			RuntimeID:  strings.TrimSpace(options.RuntimeID),
			SessionID:  transportSessionID(ctx),
			ToolCallID: toolCallID(request),
			TraceID:    traceID(request),
			Transport:  normalizedTransport(options.Transport),
		},
	}
	arguments, err := decodeArguments(
		request.Params.Name,
		options.Scope,
		request.Params.Arguments,
	)
	if err != nil {
		return call, err
	}
	call.Arguments = arguments
	if err := routeArguments(ctx, &call.Invocation, arguments, options); err != nil {
		call.Arguments = nil
		return call, err
	}
	return call, nil
}

func routeArguments(
	ctx context.Context,
	target *invocation.Invocation,
	arguments toolArguments,
	options RoutingOptions,
) error {
	if arguments, ok := arguments.(scopedWorkArguments); ok {
		return routeWork(ctx, target, options, arguments.routingScope())
	}
	switch arguments := arguments.(type) {
	case helpArguments, statusArguments:
		return nil
	case worktreeOpenArguments:
		return routeRepo(target, options.Projects, arguments.Repo)
	case worktreeListArguments:
		if arguments.Repo == "" {
			return nil
		}
		return routeRepo(target, options.Projects, arguments.Repo)
	case worktreeStatusArguments:
		return routeWorktree(ctx, target, options, arguments.WorktreeID)
	case worktreeCloseArguments:
		return routeWorktree(ctx, target, options, arguments.WorktreeID)
	case worktreeDeleteArguments:
		return routeWorktree(ctx, target, options, arguments.WorktreeID)
	case turnBeginArguments:
		if arguments.WorktreeID != "" {
			return routeWorktree(ctx, target, options, arguments.WorktreeID)
		}
		return routeRepo(target, options.Projects, arguments.Repo)
	case turnStatusArguments:
		switch {
		case arguments.TurnID != "":
			return routeTurn(ctx, target, options, arguments.TurnID, false)
		case arguments.WorktreeID != "":
			return routeWorktree(ctx, target, options, arguments.WorktreeID)
		case arguments.Repo != "":
			return routeRepo(target, options.Projects, arguments.Repo)
		default:
			return nil
		}
	case turnEndArguments:
		return routeTurn(ctx, target, options, arguments.TurnID, true)
	case diffTurnArguments:
		return routeTurn(ctx, target, options, arguments.TurnID, false)
	case diffWorktreeArguments:
		return routeWorktree(ctx, target, options, arguments.WorktreeID)
	case diffGitArguments:
		if arguments.TurnID != "" {
			return routeTurn(ctx, target, options, arguments.TurnID, false)
		}
		return routeWorktree(ctx, target, options, arguments.WorktreeID)
	case diffRevisionArguments:
		return routeRevisions(ctx, target, options, arguments.From, arguments.To)
	default:
		return result.NewError(
			result.CodeInternal,
			"decoded tool arguments have no routing policy",
		)
	}
}

func routeWork(
	ctx context.Context,
	target *invocation.Invocation,
	options RoutingOptions,
	arguments scopeArguments,
) error {
	switch options.Scope {
	case TurnScope:
		return routeTurn(ctx, target, options, arguments.TurnID, true)
	case FreestyleScope:
		return routeRepo(target, options.Projects, arguments.Repo)
	default:
		return validateScope(options.Scope)
	}
}

func routeRepo(
	target *invocation.Invocation,
	resolver *project.Resolver,
	repo string,
) error {
	if resolver == nil {
		return result.NewError(
			result.CodeInternal,
			"project resolver is not initialized",
		)
	}
	cwd, err := resolver.ResolveRepo(repo)
	if err != nil {
		return err
	}
	target.CWD = cwd
	return nil
}

func routeTurn(
	ctx context.Context,
	target *invocation.Invocation,
	options RoutingOptions,
	turnID TurnID,
	active bool,
) error {
	if options.Resources == nil {
		return result.NewError(
			result.CodeInternal,
			"lifecycle resource resolver is not initialized",
		)
	}
	var (
		scope invocation.Scope
		err   error
	)
	if active {
		scope, err = options.Resources.ResolveActiveTurn(ctx, turnID)
	} else {
		scope, err = options.Resources.ResolveTurn(ctx, turnID)
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(scope.TurnID) != string(turnID) {
		return result.NewError(
			result.CodeInternal,
			"turn resolver returned a mismatched turn scope",
		)
	}
	if err := applyResourceScope(target, options.Projects, scope); err != nil {
		return err
	}
	target.TurnID = string(turnID)
	return nil
}

func routeWorktree(
	ctx context.Context,
	target *invocation.Invocation,
	options RoutingOptions,
	worktreeID WorktreeID,
) error {
	if options.Resources == nil {
		return result.NewError(
			result.CodeInternal,
			"lifecycle resource resolver is not initialized",
		)
	}
	scope, err := options.Resources.ResolveWorktree(ctx, worktreeID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(scope.WorktreeID) != string(worktreeID) {
		return result.NewError(
			result.CodeInternal,
			"worktree resolver returned a mismatched worktree scope",
		)
	}
	if err := applyResourceScope(target, options.Projects, scope); err != nil {
		return err
	}
	target.TurnID = ""
	target.WorktreeID = string(worktreeID)
	return nil
}

func routeRevisions(
	ctx context.Context,
	target *invocation.Invocation,
	options RoutingOptions,
	from RevisionID,
	to RevisionID,
) error {
	if options.Resources == nil {
		return result.NewError(
			result.CodeInternal,
			"lifecycle resource resolver is not initialized",
		)
	}
	fromScope, err := options.Resources.ResolveRevision(ctx, from)
	if err != nil {
		return err
	}
	toScope, err := options.Resources.ResolveRevision(ctx, to)
	if err != nil {
		return err
	}

	fromInvocation := invocation.Invocation{}
	if err := applyResourceScope(&fromInvocation, options.Projects, fromScope); err != nil {
		return err
	}
	toInvocation := invocation.Invocation{}
	if err := applyResourceScope(&toInvocation, options.Projects, toScope); err != nil {
		return err
	}
	fromOwner := fromInvocation.Owner()
	toOwner := toInvocation.Owner()
	if err := invocation.RequireOwner(fromOwner, toOwner); err != nil {
		return result.NewError(
			result.CodeInvalidArgument,
			"revision endpoints belong to different owners",
		).WithField("to")
	}
	target.CWD = fromInvocation.CWD
	target.TurnID = fromInvocation.TurnID
	target.WorktreeID = fromInvocation.WorktreeID
	return nil
}

func applyResourceScope(
	target *invocation.Invocation,
	resolver *project.Resolver,
	scope invocation.Scope,
) error {
	if resolver == nil {
		return result.NewError(
			result.CodeInternal,
			"project resolver is not initialized",
		)
	}
	cwd, err := resolver.ResolveRepo(scope.WorkspaceCWD)
	if err != nil {
		return result.Wrap(
			result.CodeInternal,
			"resolve lifecycle workspace",
			err,
		)
	}
	target.CWD = cwd
	target.TurnID = strings.TrimSpace(scope.TurnID)
	target.WorktreeID = strings.TrimSpace(scope.WorktreeID)
	return nil
}

func normalizedTransport(transport string) string {
	transport = strings.TrimSpace(transport)
	if transport == "" {
		return "stdio"
	}
	return transport
}

func transportSessionID(ctx context.Context) string {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return ""
	}
	return strings.TrimSpace(session.SessionID())
}

func toolCallID(request mcp.CallToolRequest) string {
	if request.Params.Meta != nil {
		for _, key := range []string{
			"tool_call_id",
			"toolCallId",
			"call_id",
			"request_id",
		} {
			if value := correlationMetaValue(
				request.Params.Meta.AdditionalFields[key],
			); value != "" {
				return value
			}
		}
		if request.Params.Meta.ProgressToken != nil {
			if value := correlationMetaValue(
				request.Params.Meta.ProgressToken,
			); value != "" {
				return value
			}
		}
	}
	return ids.NewPrefixed("tool_")
}

func traceID(request mcp.CallToolRequest) string {
	if request.Params.Meta == nil {
		return ""
	}
	for _, key := range []string{"trace_id", "traceId"} {
		if value := correlationMetaValue(
			request.Params.Meta.AdditionalFields[key],
		); value != "" {
			return value
		}
	}
	traceparent := correlationMetaValue(
		request.Params.Meta.AdditionalFields["traceparent"],
	)
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 ||
		!validHex(parts[0], 2) ||
		strings.EqualFold(parts[0], "ff") ||
		!validNonzeroHex(parts[1], 32) ||
		!validNonzeroHex(parts[2], 16) ||
		!validHex(parts[3], 2) {
		return ""
	}
	return strings.ToLower(parts[1])
}

func correlationMetaValue(value any) string {
	var text string
	switch value := value.(type) {
	case string:
		text = value
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			text = strconv.FormatInt(integer, 10)
			break
		}
		number, err := value.Float64()
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return ""
		}
		text = strconv.FormatFloat(number, 'g', -1, 64)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ""
		}
		text = strconv.FormatFloat(value, 'g', -1, 64)
	case float32:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return ""
		}
		text = strconv.FormatFloat(float64(value), 'g', -1, 32)
	case int:
		text = strconv.Itoa(value)
	case int8:
		text = strconv.FormatInt(int64(value), 10)
	case int16:
		text = strconv.FormatInt(int64(value), 10)
	case int32:
		text = strconv.FormatInt(int64(value), 10)
	case int64:
		text = strconv.FormatInt(value, 10)
	case uint:
		text = strconv.FormatUint(uint64(value), 10)
	case uint8:
		text = strconv.FormatUint(uint64(value), 10)
	case uint16:
		text = strconv.FormatUint(uint64(value), 10)
	case uint32:
		text = strconv.FormatUint(uint64(value), 10)
	case uint64:
		text = strconv.FormatUint(value, 10)
	default:
		return ""
	}
	if len(text) > maxCorrelationMetadataBytes {
		return ""
	}
	// Correlation values are machine identifiers carried into operator-facing
	// records. Printable ASCII avoids invisible and direction-changing text.
	for index := range len(text) {
		if text[index] < 0x20 || text[index] > 0x7e {
			return ""
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return text
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for index := range value {
		character := value[index]
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') &&
			(character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

func validNonzeroHex(value string, length int) bool {
	return validHex(value, length) &&
		strings.Trim(value, "0") != ""
}
