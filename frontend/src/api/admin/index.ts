/**
 * Admin API barrel export
 * Centralized exports for all admin API modules
 */

import dashboardAPI from './dashboard'
import usersAPI from './users'
import groupsAPI from './groups'
import accountsAPI from './accounts'
import proxiesAPI from './proxies'
import redeemAPI from './redeem'
import promoAPI from './promo'
import announcementsAPI from './announcements'
import settingsAPI from './settings'
import systemAPI from './system'
import subscriptionsAPI from './subscriptions'
import usageAPI from './usage'
import geminiAPI from './gemini'
import antigravityAPI from './antigravity'
import userAttributesAPI from './userAttributes'
import opsAPI from './ops'
import errorPassthroughAPI from './errorPassthrough'
import dataManagementAPI from './dataManagement'
import apiKeysAPI from './apiKeys'
import scheduledTestsAPI from './scheduledTests'
import backupAPI from './backup'
import tlsFingerprintProfileAPI from './tlsFingerprintProfile'
import channelsAPI from './channels'
import channelMonitorAPI from './channelMonitor'
import channelMonitorTemplateAPI from './channelMonitorTemplate'
import modelMarketplaceMonitorAPI from './modelMarketplaceMonitor'
import modelMarketplaceTemplateAPI from './modelMarketplaceTemplate'
import adminPaymentAPI from './payment'
import affiliatesAPI from './affiliates'
import riskControlAPI from './riskControl'
import auditLogsAPI from './auditLogs'
import operationsAPI from './operations'

/**
 * Unified admin API object for convenient access
 */
export const adminAPI = {
  dashboard: dashboardAPI,
  users: usersAPI,
  groups: groupsAPI,
  accounts: accountsAPI,
  proxies: proxiesAPI,
  redeem: redeemAPI,
  promo: promoAPI,
  announcements: announcementsAPI,
  settings: settingsAPI,
  system: systemAPI,
  subscriptions: subscriptionsAPI,
  usage: usageAPI,
  gemini: geminiAPI,
  antigravity: antigravityAPI,
  userAttributes: userAttributesAPI,
  ops: opsAPI,
  errorPassthrough: errorPassthroughAPI,
  dataManagement: dataManagementAPI,
  apiKeys: apiKeysAPI,
  scheduledTests: scheduledTestsAPI,
  backup: backupAPI,
  tlsFingerprintProfiles: tlsFingerprintProfileAPI,
  channels: channelsAPI,
  channelMonitor: channelMonitorAPI,
  channelMonitorTemplate: channelMonitorTemplateAPI,
  modelMarketplaceMonitor: modelMarketplaceMonitorAPI,
  modelMarketplaceTemplate: modelMarketplaceTemplateAPI,
  payment: adminPaymentAPI,
  affiliates: affiliatesAPI,
  riskControl: riskControlAPI,
  auditLogs: auditLogsAPI,
  operations: operationsAPI
}

export {
  dashboardAPI,
  usersAPI,
  groupsAPI,
  accountsAPI,
  proxiesAPI,
  redeemAPI,
  promoAPI,
  announcementsAPI,
  settingsAPI,
  systemAPI,
  subscriptionsAPI,
  usageAPI,
  geminiAPI,
  antigravityAPI,
  userAttributesAPI,
  opsAPI,
  errorPassthroughAPI,
  dataManagementAPI,
  apiKeysAPI,
  scheduledTestsAPI,
  backupAPI,
  tlsFingerprintProfileAPI,
  channelsAPI,
  channelMonitorAPI,
  channelMonitorTemplateAPI,
  modelMarketplaceMonitorAPI,
  modelMarketplaceTemplateAPI,
  adminPaymentAPI,
  affiliatesAPI,
  riskControlAPI,
  auditLogsAPI,
  operationsAPI
}

export default adminAPI

// Re-export types used by components
export type { BalanceHistoryItem } from './users'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from './errorPassthrough'
export type { BackupAgentHealth, DataManagementConfig } from './dataManagement'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from './tlsFingerprintProfile'
export type { ContentModerationConfig, ContentModerationLog, ModerationMode } from './riskControl'
export type {
  AdminAuditBalanceSummary,
  AdminAuditBalanceSummaryItem,
  AdminAuditLog,
  AdminAuditLogFilters
} from './auditLogs'
export type {
  OperationArpuPoint,
  OperationBaselines,
  OperationChurnHistoryPoint,
  OperationChurnSnapshot,
  OperationChurnWaterfall,
  OperationCoreMetrics,
  OperationDistributionItem,
  OperationDistributionSnapshot,
  OperationFinancialCockpit,
  OperationFunnelStep,
  OperationModelHealth,
  OperationPlanMatrix,
  OperationProductMatrix,
  OperationPyramidLevel,
  OperationRetentionCohort,
  OperationSnapshot,
  OperationSnapshotParams,
  OperationTrendPoint,
  OperationTrialFunnel,
  OperationUserListItem,
  OperationUserLists,
  OperationUserPyramid
} from './operations'
