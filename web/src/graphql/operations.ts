import { gql } from '@apollo/client'

// =============================================================================
// FRAGMENTS
// =============================================================================

export const ROLE_FRAGMENT = gql`
  fragment RoleFields on Role {
    id
    name
    description
    isDefault
    isSystem
    createdAt
    updatedAt
    policy {
      id
      promptPolicies {
        structuralSeparation {
          enabled
          templateFormat
          forbidInstructionsInData
          markRetrievedAsUntrusted
        }
        normalization {
          enabled
          unicodeNormalization
          stripNullBytes
          removeInvisibleChars
          detectMixedEncodings
          rejectSuspiciousEncoding
        }
        inputBounds {
          enabled
          maxPromptLength
          maxPromptTokens
          maxMessageCount
          maxMessageLength
        }
        directInjectionDetection {
          enabled
          detectionMethod
          sensitivity
          onDetection
          blockThreshold
        }
        indirectInjectionDetection {
          enabled
          detectionMethod
          sensitivity
          onDetection
          blockThreshold
        }
        piiPolicy {
          enabled
          scanInputs
          scanOutputs
          categories
          onDetection
          redaction {
            placeholderFormat
            storeOriginals
            restoreInResponse
          }
        }
        contentFiltering {
          enabled
          blockedCategories
          customBlockedPatterns
          onDetection
        }
        systemPromptProtection {
          enabled
          detectExtractionAttempts
          addAntiExtractionSuffix
        }
        outputValidation {
          enabled
          enforceSchema
          detectCodeExecution
          detectSecretLeakage
          detectPIILeakage
          onViolation
        }
      }
      toolPolicies {
        allowToolCalling
        allowedTools
        blockedTools
        maxToolCallsPerRequest
        requireToolApproval
      }
      rateLimitPolicy {
        requestsPerMinute
        requestsPerHour
        requestsPerDay
        tokensPerMinute
        tokensPerHour
        tokensPerDay
      }
      modelRestrictions {
        allowedModels
        allowedProviders
        defaultModel
        maxTokensPerRequest
      }
      mcpPolicies {
        enabled
        allowToolSearch
        auditToolExecution
      }
      cachingPolicy {
        enabled
        similarityThreshold
        ttlSeconds
        maxCacheSize
        cacheStreaming
        cacheToolCalls
        excludedModels
        excludedPatterns
        trackSavings
      }
      routingPolicy {
        enabled
        strategy
        allowModelOverride
      }
      resiliencePolicy {
        enabled
        retryEnabled
        maxRetries
        retryBackoffMs
        retryBackoffMax
        retryJitter
        retryOnTimeout
        retryOnRateLimit
        retryOnServerError
        fallbackEnabled
        circuitBreakerEnabled
        circuitBreakerThreshold
        circuitBreakerTimeout
        requestTimeoutMs
      }
      budgetPolicy {
        enabled
        dailyLimitUSD
        weeklyLimitUSD
        monthlyLimitUSD
        maxCostPerRequest
        alertThreshold
        criticalThreshold
        alertWebhook
        alertEmails
        alertSlack
        onExceeded
        softLimitEnabled
        softLimitBuffer
      }
    }
  }
`

export const API_KEY_FRAGMENT = gql`
  fragment APIKeyFields on APIKey {
    id
    name
    keyPrefix
    lastUsedAt
    createdAt
    createdBy
    createdByEmail
    expiresAt
    isExpired
    revoked
    role {
      id
      name
    }
    group {
      id
      name
    }
  }
`

// =============================================================================
// AUTH
// =============================================================================

export const LOGIN = gql`
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      token
      user {
        id
        email
        name
        role
      }
      expiresAt
    }
  }
`

export const LOGOUT = gql`
  mutation Logout {
    logout
  }
`

export const ME = gql`
  query Me {
    me {
      id
      email
      name
      role
      status
      createdAt
    }
  }
`

// =============================================================================
// TENANTS (Admin Portal)
// =============================================================================

export const GET_TENANTS = gql`
  query GetTenants {
    tenants {
      id
      name
      slug
      email
      status
      tier
      createdAt
      updatedAt
    }
  }
`

export const GET_TENANT = gql`
  query GetTenant($id: ID!) {
    tenant(id: $id) {
      id
      name
      slug
      email
      status
      tier
      settings {
        defaultModel
        maxConcurrentRequests
        webhookUrl
      }
      quotas {
        maxRequestsPerMinute
        maxRequestsPerDay
        maxTokensPerDay
        maxCostPerMonthUSD
      }
      planLimits {
        maxConnectionsPerProvider
        maxIdleConnections
        maxConcurrentRequests
        maxQueuedRequests
        maxRoles
        maxAPIKeys
        maxProviders
      }
      planLimitsOverride {
        maxConnectionsPerProvider
        maxIdleConnections
        maxConcurrentRequests
        maxQueuedRequests
        maxRoles
        maxAPIKeys
        maxProviders
      }
      createdAt
      updatedAt
    }
  }
`

export const GET_TENANT_BY_SLUG = gql`
  query GetTenantBySlug($slug: String!) {
    tenantBySlug(slug: $slug) {
      id
      name
      slug
      email
      status
      tier
      planLimits {
        maxConnectionsPerProvider
        maxIdleConnections
        maxConcurrentRequests
        maxQueuedRequests
        maxRoles
        maxAPIKeys
        maxProviders
      }
      planLimitsOverride {
        maxConnectionsPerProvider
        maxIdleConnections
        maxConcurrentRequests
        maxQueuedRequests
        maxRoles
        maxAPIKeys
        maxProviders
      }
    }
  }
`

export const CREATE_TENANT = gql`
  mutation CreateTenant($input: CreateTenantInput!) {
    createTenant(input: $input) {
      id
      name
      slug
      email
      status
      tier
    }
  }
`

export const UPDATE_TENANT = gql`
  mutation UpdateTenant($id: ID!, $input: UpdateTenantInput!) {
    updateTenant(id: $id, input: $input) {
      id
      name
      email
      status
      tier
    }
  }
`

export const DELETE_TENANT = gql`
  mutation DeleteTenant($id: ID!) {
    deleteTenant(id: $id)
  }
`

// =============================================================================
// REGISTRATION
// =============================================================================

export const CREATE_REGISTRATION_REQUEST = gql`
  mutation CreateRegistrationRequest($input: CreateRegistrationRequestInput!) {
    createRegistrationRequest(input: $input) {
      id
      organizationName
      organizationEmail
      adminName
      adminEmail
      slug
      status
      requestedAt
      createdAt
    }
  }
`

export const GET_REGISTRATION_REQUESTS = gql`
  query GetRegistrationRequests($status: String) {
    registrationRequests(status: $status) {
      id
      organizationName
      organizationEmail
      adminName
      adminEmail
      slug
      status
      rejectionReason
      requestedAt
      reviewedAt
      reviewedBy
      createdAt
      updatedAt
    }
  }
`

export const APPROVE_REGISTRATION = gql`
  mutation ApproveRegistration($input: ApproveRegistrationInput!) {
    approveRegistration(input: $input) {
      id
      name
      slug
      email
      status
      tier
    }
  }
`

export const REJECT_REGISTRATION = gql`
  mutation RejectRegistration($input: RejectRegistrationInput!) {
    rejectRegistration(input: $input)
  }
`

// =============================================================================
// PROVIDERS & MODELS
// =============================================================================

export const PROVIDER_API_KEY_FRAGMENT = gql`
  fragment ProviderAPIKeyFields on ProviderAPIKey {
    id
    provider
    name
    keyPrefix
    credentialType
    priority
    enabled
    healthScore
    successCount
    failureCount
    rateLimitRemaining
    rateLimitResetAt
    requestCount
    lastUsedAt
    createdAt
    updatedAt
  }
`

export const GET_PROVIDERS = gql`
  query GetProviders {
    providers {
      provider
      enabled
      hasApiKey
      hasAccessKeys
      streamingMode
      baseUrl
      region
      regionPrefix
      resourceName
      apiVersion
      connectionSettings {
        maxConnections
        maxIdleConnections
        idleTimeoutSec
        requestTimeoutSec
        enableHTTP2
        enableKeepAlive
      }
      planCeiling {
        maxConnections
        maxIdleConnections
        idleTimeoutSec
        requestTimeoutSec
        enableHTTP2
        enableKeepAlive
      }
      apiKeys {
        ...ProviderAPIKeyFields
      }
    }
  }
  ${PROVIDER_API_KEY_FRAGMENT}
`

export const UPDATE_PROVIDER = gql`
  mutation UpdateProvider($input: UpdateProviderInput!) {
    updateProvider(input: $input) {
      provider
      enabled
      hasApiKey
      hasAccessKeys
      streamingMode
      connectionSettings {
        maxConnections
        maxIdleConnections
        idleTimeoutSec
        requestTimeoutSec
        enableHTTP2
        enableKeepAlive
      }
      planCeiling {
        maxConnections
        maxIdleConnections
        idleTimeoutSec
        requestTimeoutSec
        enableHTTP2
        enableKeepAlive
      }
    }
  }
`

export const GET_MODELS = gql`
  query GetModels {
    models {
      id
      name
      provider
      enabled
      supportsTools
      supportsStreaming
      contextLimit
      inputCostPer1M
      outputCostPer1M
    }
  }
`

export const GET_AVAILABLE_MODELS = gql`
  query GetAvailableModels {
    availableModels {
      id
      name
      provider
      enabled
      supportsTools
      contextLimit
      inputCostPer1M
      outputCostPer1M
    }
  }
`

export const REFRESH_PROVIDER_MODELS = gql`
  mutation RefreshProviderModels($provider: Provider!) {
    refreshProviderModels(provider: $provider) {
      success
      count
      message
      provider
    }
  }
`

// Multi-Key Management
export const ADD_PROVIDER_API_KEY = gql`
  mutation AddProviderAPIKey($input: AddProviderAPIKeyInput!) {
    addProviderAPIKey(input: $input) {
      ...ProviderAPIKeyFields
    }
  }
  ${PROVIDER_API_KEY_FRAGMENT}
`

export const UPDATE_PROVIDER_API_KEY = gql`
  mutation UpdateProviderAPIKey($input: UpdateProviderAPIKeyInput!) {
    updateProviderAPIKey(input: $input) {
      ...ProviderAPIKeyFields
    }
  }
  ${PROVIDER_API_KEY_FRAGMENT}
`

export const DELETE_PROVIDER_API_KEY = gql`
  mutation DeleteProviderAPIKey($id: ID!) {
    deleteProviderAPIKey(id: $id)
  }
`

// =============================================================================
// ROLES
// =============================================================================

export const GET_ROLES = gql`
  query GetRoles {
    roles {
      ...RoleFields
    }
  }
  ${ROLE_FRAGMENT}
`

export const GET_ROLE = gql`
  query GetRole($id: ID!) {
    role(id: $id) {
      ...RoleFields
    }
  }
  ${ROLE_FRAGMENT}
`

export const CREATE_ROLE = gql`
  mutation CreateRole($input: CreateRoleInput!) {
    createRole(input: $input) {
      ...RoleFields
    }
  }
  ${ROLE_FRAGMENT}
`

export const UPDATE_ROLE = gql`
  mutation UpdateRole($id: ID!, $input: UpdateRoleInput!) {
    updateRole(id: $id, input: $input) {
      ...RoleFields
    }
  }
  ${ROLE_FRAGMENT}
`

export const UPDATE_ROLE_POLICY = gql`
  mutation UpdateRolePolicy($roleId: ID!, $input: RolePolicyInput!) {
    updateRolePolicy(roleId: $roleId, input: $input) {
      id
      promptPolicies {
        piiPolicy {
          enabled
          scanInputs
          scanOutputs
          categories
          onDetection
        }
        contentFiltering {
          enabled
          blockedCategories
          onDetection
        }
        directInjectionDetection {
          enabled
          sensitivity
          onDetection
        }
        inputBounds {
          maxPromptLength
          maxMessageCount
        }
      }
      toolPolicies {
        allowToolCalling
        allowedTools
        blockedTools
      }
      rateLimitPolicy {
        requestsPerMinute
        tokensPerMinute
        tokensPerDay
      }
      modelRestrictions {
        allowedModels
        allowedProviders
        maxTokensPerRequest
      }
      mcpPolicies {
        enabled
        allowToolSearch
        auditToolExecution
      }
      cachingPolicy {
        enabled
        similarityThreshold
        ttlSeconds
        maxCacheSize
        cacheStreaming
        cacheToolCalls
        excludedModels
        excludedPatterns
        trackSavings
      }
      routingPolicy {
        enabled
        strategy
        allowModelOverride
      }
      resiliencePolicy {
        enabled
        retryEnabled
        maxRetries
        retryBackoffMs
        retryBackoffMax
        retryJitter
        retryOnTimeout
        retryOnRateLimit
        retryOnServerError
        fallbackEnabled
        circuitBreakerEnabled
        circuitBreakerThreshold
        circuitBreakerTimeout
        requestTimeoutMs
      }
      budgetPolicy {
        enabled
        dailyLimitUSD
        weeklyLimitUSD
        monthlyLimitUSD
        maxCostPerRequest
        alertThreshold
        criticalThreshold
        alertWebhook
        alertEmails
        alertSlack
        onExceeded
        softLimitEnabled
        softLimitBuffer
      }
    }
  }
`

export const DELETE_ROLE = gql`
  mutation DeleteRole($id: ID!) {
    deleteRole(id: $id)
  }
`

// =============================================================================
// GROUPS
// =============================================================================

export const GET_GROUPS = gql`
  query GetGroups {
    groups {
      id
      name
      description
      roles {
        id
        name
      }
      createdAt
      updatedAt
    }
  }
`

export const CREATE_GROUP = gql`
  mutation CreateGroup($input: CreateGroupInput!) {
    createGroup(input: $input) {
      id
      name
      description
      roles {
        id
        name
      }
    }
  }
`

export const UPDATE_GROUP = gql`
  mutation UpdateGroup($id: ID!, $input: UpdateGroupInput!) {
    updateGroup(id: $id, input: $input) {
      id
      name
      description
      roles {
        id
        name
      }
    }
  }
`

export const DELETE_GROUP = gql`
  mutation DeleteGroup($id: ID!) {
    deleteGroup(id: $id)
  }
`

// =============================================================================
// API KEYS
// =============================================================================

export const GET_API_KEYS = gql`
  query GetAPIKeys {
    apiKeys {
      ...APIKeyFields
    }
  }
  ${API_KEY_FRAGMENT}
`

export const CREATE_API_KEY = gql`
  mutation CreateAPIKey($input: CreateAPIKeyInput!) {
    createAPIKey(input: $input) {
      apiKey {
        ...APIKeyFields
      }
      secret
    }
  }
  ${API_KEY_FRAGMENT}
`

export const UPDATE_API_KEY = gql`
  mutation UpdateAPIKey($id: ID!, $input: UpdateAPIKeyInput!) {
    updateAPIKey(id: $id, input: $input) {
      ...APIKeyFields
    }
  }
  ${API_KEY_FRAGMENT}
`

export const DELETE_API_KEY = gql`
  mutation DeleteAPIKey($id: ID!) {
    deleteAPIKey(id: $id)
  }
`

// =============================================================================
// DASHBOARD & ANALYTICS
// =============================================================================

export const GET_DASHBOARD = gql`
  query GetDashboard {
    dashboard {
      totalRequests
      totalTokens
      totalCostUSD
      avgLatencyMs
      errorRate
      requestsByHour {
        hour
        requests
        tokens
      }
      costTrend {
        date
        cost
      }
      topModels {
        model
        requests
        tokens
        cost
      }
      providerBreakdown {
        provider
        requests
        percentage
      }
      apiKeyBreakdown {
        apiKeyId
        apiKeyName
        requests
        tokens
        cost
        percentage
      }
    }
  }
`

export const GET_REQUEST_LOGS = gql`
  query GetRequestLogs($filter: RequestLogFilter, $first: Int, $after: String) {
    requestLogs(filter: $filter, first: $first, after: $after) {
      edges {
        id
        model
        provider
        status
        inputTokens
        outputTokens
        latencyMs
        costUSD
        apiKeyName
        errorCode
        errorMessage
        createdAt
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        endCursor
      }
      totalCount
    }
  }
`

export const GET_REQUEST_LOG_DETAIL = gql`
  query GetRequestLogDetail($id: ID!) {
    requestLog(id: $id) {
      id
      model
      provider
      status
      inputTokens
      outputTokens
      latencyMs
      costUSD
      apiKeyName
      errorCode
      errorMessage
      prompt
      response
      metadata
      createdAt
    }
  }
`

export const GET_COST_ANALYSIS = gql`
  query GetCostAnalysis($startDate: DateTime, $endDate: DateTime) {
    costAnalysis(startDate: $startDate, endDate: $endDate) {
      totalCost
      periodStart
      periodEnd
      dailyCosts {
        date
        cost
      }
      costByProvider {
        provider
        cost
        percentage
      }
      costByModel {
        model
        cost
        requests
      }
      projectedMonthlyCost
      budgetUtilization
    }
  }
`

export const GET_PERFORMANCE = gql`
  query GetPerformance($startDate: DateTime, $endDate: DateTime) {
    performance(startDate: $startDate, endDate: $endDate) {
      avgLatencyMs
      p50LatencyMs
      p95LatencyMs
      p99LatencyMs
      successRate
      errorRate
      modelPerformance {
        model
        avgLatencyMs
        successRate
        requestCount
      }
    }
  }
`

// =============================================================================
// BUDGET ALERTS
// =============================================================================

export const GET_BUDGET_ALERTS = gql`
  query GetBudgetAlerts {
    budgetAlerts {
      id
      name
      type
      threshold
      thresholdType
      period
      enabled
      lastTriggeredAt
      createdAt
    }
  }
`

export const CREATE_BUDGET_ALERT = gql`
  mutation CreateBudgetAlert($input: CreateBudgetAlertInput!) {
    createBudgetAlert(input: $input) {
      id
      name
      type
      threshold
      period
      enabled
    }
  }
`

export const UPDATE_BUDGET_ALERT = gql`
  mutation UpdateBudgetAlert($id: ID!, $input: UpdateBudgetAlertInput!) {
    updateBudgetAlert(id: $id, input: $input) {
      id
      name
      threshold
      enabled
    }
  }
`

export const DELETE_BUDGET_ALERT = gql`
  mutation DeleteBudgetAlert($id: ID!) {
    deleteBudgetAlert(id: $id)
  }
`

// =============================================================================
// USERS
// =============================================================================

export const GET_USERS = gql`
  query GetUsers {
    users {
      id
      email
      name
      role
      status
      createdAt
      createdBy
      createdByEmail
      lastLoginAt
    }
  }
`

export const CREATE_USER = gql`
  mutation CreateUser($email: String!, $name: String!, $password: String!, $role: String!) {
    createUser(email: $email, name: $name, password: $password, role: $role) {
      id
      email
      name
      role
      createdBy
      createdByEmail
    }
  }
`

export const DELETE_USER = gql`
  mutation DeleteUser($id: ID!) {
    deleteUser(id: $id)
  }
`

// =============================================================================
// AUDIT LOGS
// =============================================================================

export const GET_AUDIT_LOGS = gql`
  query GetAuditLogs(
    $filter: AuditLogFilter
    $limit: Int
    $offset: Int
  ) {
    auditLogs(filter: $filter, limit: $limit, offset: $offset) {
      items {
        id
        timestamp
        action
        resourceType
        resourceId
        resourceName
        actorId
        actorEmail
        actorType
        ipAddress
        userAgent
        details
        oldValue
        newValue
        status
        errorMessage
      }
      totalCount
      hasMore
    }
  }
`

// =============================================================================
// TOOL POLICY
// =============================================================================

export const DISCOVERED_TOOL_FRAGMENT = gql`
  fragment DiscoveredToolFields on DiscoveredTool {
    id
    name
    description
    schemaHash
    parameters
    category
    firstSeenAt
    lastSeenAt
    firstSeenBy
    seenCount
    createdAt
    updatedAt
  }
`

export const GET_ROLE_TOOL_PERMISSIONS = gql`
  query GetRoleToolPermissions($roleId: ID!) {
    roleToolPermissions(roleId: $roleId) {
      tool {
        ...DiscoveredToolFields
      }
      status
      decidedBy
      decidedByEmail
      decidedAt
      decisionReason
    }
  }
  ${DISCOVERED_TOOL_FRAGMENT}
`

export const GET_PENDING_TOOLS = gql`
  query GetPendingTools {
    pendingTools {
      ...DiscoveredToolFields
    }
  }
  ${DISCOVERED_TOOL_FRAGMENT}
`

export const GET_DISCOVERED_TOOLS = gql`
  query GetDiscoveredTools(
    $filter: DiscoveredToolFilter
    $limit: Int
    $offset: Int
  ) {
    discoveredTools(filter: $filter, limit: $limit, offset: $offset) {
      items {
        ...DiscoveredToolFields
      }
      totalCount
      hasMore
    }
  }
  ${DISCOVERED_TOOL_FRAGMENT}
`

export const SET_TOOL_PERMISSION = gql`
  mutation SetToolPermission($input: SetToolPermissionInput!) {
    setToolPermission(input: $input) {
      id
      status
      decidedBy
      decidedByEmail
      decidedAt
      decisionReason
      tool {
        id
        name
      }
    }
  }
`

export const SET_TOOL_PERMISSIONS_BULK = gql`
  mutation SetToolPermissionsBulk($input: SetToolPermissionsBulkInput!) {
    setToolPermissionsBulk(input: $input) {
      id
      status
      tool {
        id
        name
      }
    }
  }
`

export const APPROVE_ALL_PENDING_TOOLS = gql`
  mutation ApproveAllPendingTools($roleId: ID!) {
    approveAllPendingTools(roleId: $roleId)
  }
`

export const DENY_ALL_PENDING_TOOLS = gql`
  mutation DenyAllPendingTools($roleId: ID!) {
    denyAllPendingTools(roleId: $roleId)
  }
`

export const REMOVE_ALL_PENDING_TOOLS = gql`
  mutation RemoveAllPendingTools($roleId: ID!) {
    removeAllPendingTools(roleId: $roleId)
  }
`

export const DELETE_DISCOVERED_TOOL = gql`
  mutation DeleteDiscoveredTool($id: ID!) {
    deleteDiscoveredTool(id: $id)
  }
`

// =============================================================================
// MCP TOOLS VISIBILITY
// =============================================================================

export const GET_MCP_SERVERS_WITH_TOOLS = gql`
  query GetMCPServersWithTools($roleId: ID!) {
    mcpServersWithTools(roleId: $roleId) {
      server {
        id
        name
        description
        status
        toolCount
      }
      tools {
        tool {
          id
          serverId
          serverName
          name
          description
          category
        }
        visibility
        decidedBy
        decidedAt
      }
      stats {
        totalTools
        allowedCount
        searchCount
        deniedCount
      }
    }
  }
`

export const SET_MCP_PERMISSION = gql`
  mutation SetMCPPermission($input: SetMCPPermissionInput!) {
    setMCPPermission(input: $input) {
      id
      roleId
      serverId
      toolId
      visibility
      decidedBy
      decidedAt
    }
  }
`

export const BULK_SET_MCP_VISIBILITY = gql`
  mutation BulkSetMCPVisibility($roleId: ID!, $serverId: ID!, $visibility: MCPToolVisibility!) {
    bulkSetMCPVisibility(roleId: $roleId, serverId: $serverId, visibility: $visibility)
  }
`

// =============================================================================
// ADVANCED METRICS
// =============================================================================

export const GET_ADVANCED_METRICS = gql`
  query GetAdvancedMetrics {
    advancedMetrics {
      cache {
        hits
        misses
        hitRate
        tokensSaved
        costSaved
        avgLatencyMs
        entries
      }
      routing {
        decisions
        strategyDistribution {
          strategy
          count
        }
        modelSwitches {
          fromModel
          toModel
          count
        }
        failures
      }
      resilience {
        circuitBreakers {
          provider
          state
          failures
        }
        retryAttempts
        fallbackInvocations
        fallbackSuccessRate
      }
      providerHealth {
        providers {
          provider
          model
          healthScore
          successRate
          p95LatencyMs
          requests
        }
      }
    }
  }
`

export const GET_CACHE_METRICS = gql`
  query GetCacheMetrics {
    cacheMetrics {
      hits
      misses
      hitRate
      tokensSaved
      costSaved
      avgLatencyMs
      entries
    }
  }
`

export const GET_ROUTING_METRICS = gql`
  query GetRoutingMetrics {
    routingMetrics {
      decisions
      strategyDistribution {
        strategy
        count
      }
      modelSwitches {
        fromModel
        toModel
        count
      }
      failures
    }
  }
`

export const GET_RESILIENCE_METRICS = gql`
  query GetResilienceMetrics {
    resilienceMetrics {
      circuitBreakers {
        provider
        state
        failures
      }
      retryAttempts
      fallbackInvocations
      fallbackSuccessRate
    }
  }
`

export const GET_PROVIDER_HEALTH_METRICS = gql`
  query GetProviderHealthMetrics {
    providerHealthMetrics {
      providers {
        provider
        model
        healthScore
        successRate
        p95LatencyMs
        requests
      }
    }
  }
`

// =============================================================================
// AGENT DASHBOARD
// =============================================================================

export const GET_AGENT_DASHBOARD = gql`
  query GetAgentDashboard($apiKeyId: ID!, $startTime: DateTime!, $endTime: DateTime!) {
    agentDashboard(apiKeyId: $apiKeyId, startTime: $startTime, endTime: $endTime) {
      providerModelUsage {
        provider
        model
        requestCount
        tokenCount
        costUsd
      }
      tokenMetrics {
        totalInput
        totalOutput
        totalThinking
        totalCost
        byModel {
          model
          inputTokens
          outputTokens
          thinkingTokens
          costUsd
        }
      }
      cacheMetrics {
        totalHits
        totalMisses
        hitRate
        tokensSaved
        costSaved
      }
      toolCallMetrics {
        totalCalls
        successCount
        failureCount
        successRate
        byTool {
          toolName
          successCount
          failureCount
          totalCount
        }
      }
      riskAssessment {
        overallRiskScore
        riskLevel
        policyViolations {
          violationType
          count
          avgSeverity
        }
        recentViolations {
          id
          apiKeyId
          policyId
          policyName
          violationType
          severity
          message
          timestamp
          metadata
        }
        recommendations
      }
    }
  }
`

