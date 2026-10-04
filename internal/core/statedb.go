/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

import (
	"context"

	"renop/internal/config"
	"renop/internal/mail"
)

// SystemDB provides database statistics, migrations, and instance repository configuration.
type SystemDB interface {
	MigrateLegacyUUIDs(ctx context.Context) (int64, error)
	GetDatabaseStats() (*DatabaseStats, error)
	CleanupUnimportantData() (*DatabaseCleanupResult, error)
	GetRepositorySettings() (*config.MavenSettings, error)
	SaveRepositorySettings(settings config.MavenSettings) error
}

// TokenDB manages named access tokens.
type TokenDB interface {
	GetTokenByName(name string) (*AccessToken, error)
	GetTokenByEmail(email string) (*AccessToken, error)
	GetTokenBySecret(secret string) (*AccessToken, error)
	GetAllTokens() ([]*AccessToken, error)
	UpdateToken(name string, updateFn func(*AccessToken)) error
	CountTokens() (uint64, error)
	SaveToken(token *AccessToken) error
	CreateToken(token *AccessToken, nickname string, changedAt int64) error
	DeleteToken(name string) error
	RenameToken(oldName, newName string, token *AccessToken) error
	SearchTokenNames(prefix string, limit int, now int64, includePrivate bool) ([]string, error)
}

// AccountDB manages account profiles, bans, retirement lifecycle, and avatars.
type AccountDB interface {
	SetAccountBan(username string, ban *AccountBan, banIPs ...bool) error
	GetAccountBanStatus(username string) (*AccountBanStatus, error)
	IsIPBanned(ip string) (bool, error)
	BanAccountIP(username, ip string) error
	UnbanAccountIP(username, ip string) error
	IsAccountIPBanned(username, ip string) (bool, error)
	ListAccountBannedIPs(username string) ([]string, error)
	GetAccountRetirementPlan(username string) (*AccountRetirementPlan, error)
	RetireAccount(username string, retiredAt int64) error
	GetAccountRetirementStatus(username string) (*AccountRetirementStatus, error)
	ReleaseRetiredAccountEmail(username string, releasedAt int64) error
	PurgeRetiredAccountAuditLogs(username string, purgedAt int64) error
	CleanupRetiredAccountData(now int64, limit int) error
	SetUserProfilePrivacy(username, session, userID string, private bool) error
	GetUserProfile(username string) (*UserProfile, error)
	GetUserProfileByID(userID string) (*UserProfile, error)
	GetUserProfiles(usernames []string) (map[string]*UserProfile, error)
	GetAccountNamesByIDs(userIDs []string) ([]string, error)
	GetUserAvatar(username string) (*UserAvatar, error)
	PutUserAvatar(username string, avatar *UserAvatar) error
	DeleteUserAvatar(username string) error
	ListUserPackageMemberships(userID, format, viewer string, moderatedRepositories []string) ([]*UserPackageMembership, error)
	UpdateUserProfileLinks(username string, links UserProfileLinks, updatedAt int64) (*UserProfile, error)
	UpdateUserProfile(oldUsername, newUsername, nickname string, token *AccessToken, changedAt int64, changes AccountTokenChanges) (*UserProfile, error)
	SetUserLocale(username, session, language, userID string) error
	GetEmailLocale(email string) (string, error)
}

// APITokenDB manages user-scoped granular API tokens.
type APITokenDB interface {
	ListAPITokens(username string) ([]*APIToken, error)
	CreateAPIToken(username string, token *APIToken, secretHash string) error
	UpdateAPIToken(username, tokenID, name string, scopes []string, targets map[string][]string) (*APIToken, error)
	RotateAPIToken(username, tokenID, newSecretHash string) (*APIToken, error)
	DeleteAPIToken(username, tokenID string) error
	SetAPITokenDisabled(username, tokenID string, disabled bool) error
	GetAPITokenByHash(secretHash, username string) (*APITokenCredential, error)
	CountAPITokens(username string) (int, error)
	CountAPITokensByUsername() (map[string]int, error)
}

// SessionDB manages browser and API session lifetimes and OAuth grants.
type SessionDB interface {
	GetSession(sessionToken string) (*Session, error)
	SaveSession(session *Session, sessionToken string) error
	GetSessionOAuthGrant(sessionToken string) (*SessionOAuthGrant, error)
	RevokeOAuthSessions(event OAuthRevocation, now int64) ([]string, error)
	UpdateSessionLastActive(sessionToken string, lastActive int64) error
	DeleteSession(sessionToken string) error
	DeleteSessionsByUsername(username string) error
	ListUserSessions(username, currentSessionToken string) ([]SessionDto, error)
	ListActiveUserSessions(username string, beforeCreatedAt int64, beforeID string, limit int, now int64) ([]SessionDto, error)
	DeleteExpiredSessions(minActiveTimestamp int64) error
	DeleteUserSessionByPublicID(username, publicID, currentSessionToken string) (token string, revoked bool, wasCurrent bool, err error)
	DeleteOtherUserSessions(username, keepSessionToken string) (tokens []string, err error)
	GetActiveSessions(minActiveTimestamp int64) ([]SessionDBDto, error)
	UpdateSessionsUsername(oldUsername, newUsername string) error
}

// FidoDB manages WebAuthn / FIDO2 security keys and sign counts.
type FidoDB interface {
	ListFidoDevices(username string) ([]*FidoDevice, error)
	GetFidoDeviceByCredentialID(credentialID []byte) (*FidoDevice, error)
	SaveFidoDevice(device *FidoDevice) error
	UpdateFidoSignCount(credentialID []byte, signCount uint32) error
	UpdateFidoDeviceState(credentialID []byte, signCount uint32, backupState bool, backupEligible bool) error
	DeleteFidoDevice(username, deviceID string) error
	DeleteFidoDevicesByUsername(username string) error
}

// AuthSecurityDB manages passwords, multi-factor authentication, recovery codes, registrations, and OAuth identities.
type AuthSecurityDB interface {
	GetAccountSecurity(username string) (*AccountSecurity, error)
	GetMFAState(username string) (*MFAState, error)
	UpdateMFA(username, snapshot, secret string, passkey bool, lastStep int64, keepSession string) error
	ConsumeMFACode(username, revision string, step, now int64) error
	PasswordLoginEnabled(username string) (bool, error)
	UpdateAccountEmail(username, email string, updatedAt int64) (*AccountSecurity, error)
	QueueAccountEmailChange(username, session string, job *mail.Job, codeHash, key, ip string, rate mail.Rate, alias ...bool) error
	DeleteAccountEmailAlias(username, session, email string, now int64) (*AccountSecurity, error)
	ConfirmAccountEmailChange(username, session, email, codeHash string, now int64) (*AccountSecurity, error)
	UpdateAccountEmailFromSession(username, session, email, snapshot string, now int64, providerEmails ...ProviderEmail) (*AccountSecurity, error)
	SetPasswordLoginEnabled(username string, enabled bool, updatedAt int64) (*AccountSecurity, error)
	SetAccountPassword(username, passwordHash string, updatedAt int64) error
	ChangeAccountPassword(change PasswordChange) error
	ReplaceRecoveryCodes(username string, codes []RecoveryCodeHash) error
	GetRecoveryCodes(identifier string, selectorHashes []string) (string, []RecoveryCodeRecord, error)
	QueueEmailPasswordReset(job *mail.Job, codeHash, key, ip string, rate mail.Rate) (bool, error)
	ResetPasswordWithEmailCode(email, codeHash, passwordHash string, updatedAt int64) (string, error)
	ResetPasswordWithRecoveryCodes(identifier string, selectorHashes []string, passwordHash string, updatedAt int64) (string, error)
	CleanRegistrations(now int64) error
	CheckRegistrationIP(ipHash string, cfg config.RegistrationConfig, now int64) error
	BeginRegistration(pending *PendingRegistration, job *mail.Job, key, ip string, rate mail.Rate, cfg config.RegistrationConfig) error
	QueueProviderRegistrationEmail(idHash, ipHash, codeHash string, job *mail.Job, key, ip string, rate mail.Rate, cfg config.RegistrationConfig) error
	GetPendingRegistration(idHash string, now int64) (*PendingRegistration, error)
	RegisterAccount(request AccountRegistration, cfg config.RegistrationConfig, now int64) (*RegistrationProfile, error)
	GetGitHubIdentity(username string) (*GitHubIdentity, error)
	GetOAuthIdentities(username string) ([]OAuthIdentity, error)
	GetOAuthIdentity(identity OAuthIdentity) (*OAuthIdentity, error)
	RefreshOAuthIdentity(userID string, identity OAuthIdentity, now int64) error
	LinkOAuthIdentity(username, session, snapshot string, identity OAuthIdentity, now int64) error
	DeleteOAuthIdentity(username, session, provider string, now int64) error
	GetGitHubIdentityByProviderID(githubUserID int64) (*GitHubIdentity, error)
	StoreGitHubIdentity(userID string, githubUserID int64, githubLogin string, principals []GitHubPrincipal, authorizedAt int64, emails ...ProviderEmail) error
	LinkGitHubIdentity(username, session, snapshot string, githubUserID int64, githubLogin string, principals []GitHubPrincipal, authorizedAt int64, emails ...ProviderEmail) error
	RefreshGitHubIdentity(userID string, githubUserID int64, githubLogin string, principals []GitHubPrincipal, authorizedAt int64, emails ...ProviderEmail) error
	DeleteGitHubIdentity(username string) error
	GetRecentGitHubPrincipal(username, login string, authorizedAfter int64) (*GitHubPrincipal, error)
}

// GPGDB manages GPG public keys, detached signatures, and asynchronous release verification.
type GPGDB interface {
	FindGPGPublicKeys(identifier string) ([]*GPGPublicKey, error)
	GetGPGPublicKey(fingerprint string) (*GPGPublicKey, error)
	ListUserGPGKeys(username string) ([]*UserGPGKey, error)
	RegisterUserGPGKey(username, requestedID string, key *GPGPublicKey, aliases []string) error
	RefreshGPGPublicKey(key *GPGPublicKey, aliases []string) error
	DeleteUserGPGKey(username, fingerprint string) error
	SaveGPGSignature(signature *GPGSignature) error
	GetGPGSignature(artifactKey string) (*GPGSignature, error)
	GetGPGSignatures(artifactKeys []string) ([]*GPGSignature, error)
	DeleteGPGSignature(artifactKey string) error
	DeleteGPGSignaturesByPrefix(repository, artifactPathPrefix string) error
	DeleteGPGSignaturesByRepository(repository string) error
	SaveGPGRelease(release *GPGRelease) error
	GetActiveGPGRelease(activeKey string) (*GPGRelease, error)
	ClaimNextGPGRelease(optionalReadyBefore int64) (*GPGRelease, error)
	ListGPGReleases(username string, limit, offset int) ([]*GPGRelease, int, error)
	ListPendingGPGReleases() ([]*GPGRelease, error)
	CountPendingGPGReleases(username string) (int, int, error)
	ResetValidatingGPGReleases() error
}

// AuditDB manages persistent security audit log entries.
type AuditDB interface {
	SaveAuditLog(entry *AuditLogEntry) error
	GetAuditLogs(username string, limit, offset int) ([]*AuditLogEntry, int, error)
	FilterAuditLogs(filter AuditLogFilter, limit, offset int) ([]*AuditLogEntry, int, error)
	DeleteAuditLogsByUsername(username string) error
	ClearGlobalAuditLogs() error
	CleanExpiredAuditLogs(retentionDays int, maxRows int) error
}

// MailDB manages durable mail queues, provider leases, and accounting.
type MailDB interface {
	AcquireMailLease(owner string, now int64) (mail.Control, bool, error)
	ReleaseMailLease(owner string) error
	QueueMailJob(job *mail.Job, key, ip string, rate mail.Rate) (bool, error)
	GetMailJob(id, key string) (*mail.Job, error)
	ListMailJobs(userID, status, key string, limit, offset int) ([]*mail.Job, int, error)
	NextMailJob(key string, now int64) (*mail.Job, error)
	LoadMailAccount(id, key string) (mail.AccountState, error)
	SaveMailAccount(owner, id, key string, state mail.AccountState, now int64) error
	SaveMailAttempt(owner, key string, job *mail.Job, state *mail.AccountState, nextSendAt int64) error
	RecoverMailAttempts(owner string, now int64) error
	AdvanceMailAuditCursor(owner string, cursor, now int64) error
	MailAuditEvents(cursor int64) ([]*AuditLogEntry, error)
	MailMessageEvents(since int64) ([]*UserMessage, error)
	AcknowledgeMailMessage(id string, now int64) error
	PreviousMailLoginIP(username string, before int64) (string, error)
	CleanMailData(now int64, accountIDs []string) error
}

// MessageDB manages user notification messages and actionable invitations.
type MessageDB interface {
	SaveMessages(messages []*UserMessage) error
	SaveMessageIfAbsent(message *UserMessage) (bool, error)
	ListMessages(username string, limit int, beforeCreatedAt int64, beforeID string, now int64, sessionID string) ([]*UserMessage, error)
	CountUnreadMessages(username string, now int64, sessionID string) (int, error)
	GetUserMessage(id, username string, now int64, sessionID string) (*UserMessage, error)
	MarkMessageRead(id, username string, readAt int64, sessionID string) (bool, error)
	MarkAllMessagesRead(username string, readAt int64, sessionID string) (int64, error)
	TransitionMessageAction(id, username, expectedStatus, newStatus string, actedAt int64) (bool, error)
	DeleteUserMessage(id, username string, sessionID string) (bool, error)
	DeleteUserMessages(username string, sessionID string) (int64, error)
	DeleteMessagesByDedupeKey(dedupeKey string) (int64, error)
}

// PublicationQuotaDB manages publication rate limit quotas and reservation transactions.
type PublicationQuotaDB interface {
	GetPublicationQuotaStatus(subject PublicationQuotaSubject, defaults PublicationQuotaLimits, now int64) (*PublicationQuotaStatus, error)
	SetPublicationQuotaOverride(subject PublicationQuotaSubject, override PublicationQuotaOverride, updatedAt int64) error
	ReservePublicationQuota(subject PublicationQuotaSubject, defaults PublicationQuotaLimits, delta PublicationQuotaDelta, now, expiresAt int64) (*PublicationQuotaReservation, error)
	CommitPublicationQuotaReservation(id string, committedAt int64) error
	ReleasePublicationQuotaReservation(id string) error
	CleanExpiredPublicationQuotaReservations(now int64) error
}

// SuperTeamDB manages multi-repository organizational teams, limits, and team invitations.
type SuperTeamDB interface {
	GetSuperTeamLimitStatus(username string, globalCreateLimit, globalJoinLimit int) (*SuperTeamLimitStatus, error)
	SetSuperTeamLimitOverride(username string, createLimit, joinLimit *int, updatedAt int64) error
	CreateSuperTeam(team *SuperTeam, owner string, globalCreateLimit, globalJoinLimit int) error
	ListSuperTeams(username string, administrator bool, limit, offset int) ([]*SuperTeam, int, error)
	ListVisibleUserSuperTeams(userID, viewer string, administrator, moderator bool, limit, offset int) ([]*UserSuperTeamMembership, int, error)
	ListSuperTeamResources(options SuperTeamResourceListOptions) ([]*SuperTeamResource, int, error)
	ListManageableSuperTeams(username string, minimumRole, limit, offset int) ([]*SuperTeam, int, error)
	GetSuperTeamRole(prefix, username string) (int, error)
	GetSuperTeamDetails(prefix, username string, administrator, moderator bool) (*SuperTeamDetails, error)
	GetPublicSuperTeamDetails(prefix, username string, administrator, moderator bool) (*SuperTeamDetails, error)
	ListSuperTeamReviewerNames(prefix string) ([]string, error)
	UpdateSuperTeam(prefix, actor, name, description string, links PublicLinks, administrator bool, updatedAt int64) error
	DeleteSuperTeam(prefix, actor string, administrator bool, actedAt int64) error
	CreateSuperTeamInvitations(invitations []*SuperTeamInvitation, messages []*UserMessage) error
	ForceAddSuperTeamMembers(prefix, actor string, usernames []string, level, globalCreateLimit, globalJoinLimit int, actedAt int64) error
	RespondSuperTeamInvitation(id, recipient string, accept bool, globalJoinLimit int, actedAt int64) error
	SetSuperTeamMemberLevel(prefix, actor, target string, level int, administrator bool) error
	SetSuperTeamMemberVisibility(prefix, username string, visible bool) error
	RemoveSuperTeamMember(prefix, actor, target string, administrator bool, actedAt int64) error
	CleanExpiredSuperTeamInvitations(now int64) error
	CreateSuperTeamTransferReview(request SuperTeamTransferRequest, actor string, administrator bool, createdAt int64) (*ReviewTask, error)
}

// TicketDB manages moderation review tasks, dispute tickets, and conversation messages.
type TicketDB interface {
	CreateOrUpdatePublicationReview(request PublicationReviewRequest) (*PublicationReviewResult, error)
	ListReviewTasks(options ReviewTaskListOptions) ([]*ReviewTask, int, error)
	TransitionTicket(id, actor, session string, action TicketAction, at int64) (*ReviewTask, error)
	CreateTicket(request TicketRequest, actor, session string, at int64) (*ReviewTask, error)
	GetTicket(id, actor string) (*ReviewTask, error)
	AddTicketMessage(msg *TicketMessage, actor, session string) error
	ListTicketMessages(taskID, before string, limit int) ([]*TicketMessage, string, error)
	GetReviewTask(id string) (*ReviewTask, error)
	ListReviewTaskFiles(id string) ([]*ReviewFile, error)
	GetReviewTaskPayload(id string) ([]byte, error)
	ListPendingPublicationReviewFiles() ([]*ReviewFile, error)
	IsPublicationReviewPathPending(repository, path string) (bool, error)
	HasPendingPublicationReviews(repository string) (bool, error)
	ListPublicationReviews(repository, resourceType, resourceName string) ([]*ReviewTask, error)
	AdvancePackageCreationReview(id, actor string, advancedAt int64) (*ReviewTask, error)
	DecideReviewTask(id, actor, decision, reason string, decidedAt int64) (*ReviewTask, error)
	CancelReviewTask(id, actor string, cancelledAt int64) (*ReviewTask, error)
	CreateMavenRestoreReview(repository, groupID, artifactID, actor, session string, createdAt int64) (*ReviewTask, error)
	CreateGitRepoRegistrationTask(repository, targetRepo, superTeamPrefix, actor, session string, createdAt int64) (*ReviewTask, error)
	CreateGitRepoQuotaTask(repository, targetRepo, superTeamPrefix, actor, session string, payload []byte, cooldownHours int, createdAt int64) (*ReviewTask, error)
	GetLastGitRepoQuotaRequestTime(repository, targetRepo string) (int64, error)
}

// MavenDB manages Maven domain ownership, verification health, publication records, and domain memberships.
type MavenDB interface {
	CreateMavenDomain(domain *MavenDomain, owner string) error
	ListMavenDomains(username string, includeAll bool) ([]*MavenDomain, error)
	ListManagedMavenDomains(options MavenDomainListOptions) ([]*MavenDomain, int, error)
	ListMavenRepositoryDomains(repository, username string, moderator bool) ([]*MavenDomain, error)
	SearchMavenRepositoryDomains(repository, query, username string, moderator bool, limit int) ([]*MavenDomain, int, error)
	GetMavenDomainDetails(domain, username string) (*MavenDomainDetails, error)
	ReserveMavenVerificationAttempt(domain, actor string, administrator bool, checkedAt, minimumPrevious int64) error
	MarkMavenDomainVerified(domain, code string, verifiedAt int64, health *MavenDomainHealth) error
	ListMavenDomainHealthChecks(now int64, limit int) ([]*MavenDomain, error)
	RecordMavenDomainHealth(expected *MavenDomain, health *MavenDomainHealth, releaseAt int64, code string) error
	ReserveMavenRedemptionAttempt(domain, actor, session string, checkedAt, minimumPrevious int64) error
	RedeemMavenDomain(expected *MavenDomain, health *MavenDomainHealth, actor, session string) error
	CloseMavenDomain(domain, actor string, administrator bool, closedAt int64) error
	ForceDeleteMavenDomain(domain, actor string) error
	ReviewMavenDomainClaim(expected *MavenDomain, health *MavenDomainHealth, actor, decision string, reviewedAt int64) error
	RecordMavenReleasedPublication(expected *MavenDomain, artifact *MavenArtifact, version *MavenVersion) error
	HasMavenMembership(username string) (bool, error)
	RecordMavenPublication(artifact *MavenArtifact, version *MavenVersion) error
	RecordMavenMirrorPublication(artifact *MavenArtifact, version *MavenVersion) error
	MavenArtifactExists(repository, groupID, artifactID string) (bool, error)
	ListMavenArtifacts(repository, domain, query string, limit, offset int) ([]*MavenArtifact, int, error)
	ListMavenDomainArtifacts(repositories []string, domain string, limit, offset int) ([]*MavenArtifact, int, error)
	ListReadableMavenArtifacts(repositories []string, domain, query, username string, moderated []string, limit, offset int) ([]*MavenArtifact, int, error)
	GetMavenArtifactDetails(repository, groupID, artifactID string) (*MavenArtifactDetails, error)
	GetMavenArtifactTeamAccess(repository, groupID, artifactID, username string) (string, bool, int, error)
	IsMavenArtifactMember(repository, groupID, artifactID, username string) (bool, error)
	UpdateMavenArtifactDescription(repository, groupID, artifactID, description string) error
	UpdateMavenArtifactReadme(repository, groupID, artifactID, readme string) error
	DeleteMavenVersionMetadata(repository, groupID, artifactID, version string) error
	RollbackMavenPublication(repository, groupID, artifactID, version string) error
	DeleteMavenRepository(repository string) error
	EnsureImportedMavenDomain(domain *MavenDomain) error
	EnsureMirroredMavenDomain(domain string, createdAt int64) error
	IsMavenRepositoryUpgraded(repository string) (bool, error)
	MarkMavenRepositoryUpgraded(repository string, completedAt int64) error
	CreateMavenInvitations(invitations []*MavenInvitation, messages []*UserMessage) error
	ForceAddMavenMembers(domain, actor string, usernames []string, level int) error
	RespondMavenInvitation(id, recipient string, accept bool, actedAt int64) error
	SetMavenMemberLevel(domain, actor, username string, level int) error
	RemoveMavenMember(domain, actor, username string) error
}

// CargoDB manages Cargo package metadata, versions, and crate permissions.
type CargoDB interface {
	GetCargoPackage(repository, normalizedName string) (*CargoPackage, error)
	CargoHasPublishedVersions(repository, normalizedName string) (bool, error)
	GetCargoPackageDetails(repository, normalizedName, username string) (*CargoPackageDetails, error)
	ListCargoPackages(repository, username string, administrator bool) ([]*CargoPackage, error)
	SearchCargoPackages(repository, query, username string, moderator bool, limit, offset int) ([]*CargoPackage, int, error)
	HasCargoPackageMembership(repository, normalizedName, username string) (bool, error)
	HasCargoMembership(repository, username string) (bool, error)
	RecordCargoPublication(pkg *CargoPackage, version *CargoVersion, username string) error
	RollbackCargoPublicationReview(repository, normalizedName, version string, previous *CargoPackage) error
	RecordCargoMirrorPublication(pkg *CargoPackage, version *CargoVersion) error
	SetCargoVersionYanked(repository, normalizedName, version string, yanked, administrator bool) error
	DeleteCargoVersion(repository, normalizedName, version string) error
	SetCargoPackageArchived(repository, normalizedName string, archived, administrator bool) error
	DeleteCargoPackage(repository, normalizedName string, actedAt int64) error
	DeleteCargoRepository(repository string, actedAt int64) error
	CreateCargoInvitations(invitations []*CargoInvitation, messages []*UserMessage) error
	RespondCargoInvitation(id, recipient, repository string, accept bool, actedAt int64) error
	SetCargoMemberLevel(repository, normalizedName, actor, username string, level int) error
	RemoveCargoMember(repository, normalizedName, actor, username string) error
	RemoveCargoMembers(repository, normalizedName, actor string, usernames []string) error
	ForceAddCargoMembers(repository, normalizedName, crateName, actor string, usernames []string, level int) error
}

// NPMDB manages npm packages, dist-tags, packument updates, and package memberships.
type NPMDB interface {
	CreateNPMPackage(repository, packageName, owner string, private bool, createdAt int64) (*NPMPackage, error)
	CreateNPMPackageForTeam(repository, packageName, owner, superTeamPrefix string, private bool, createdAt int64) (*NPMPackage, error)
	GetNPMPackage(repository, packageName string) (*NPMPackage, error)
	GetNPMPackageAccess(repository, packageName, username string) (exists, private, publishEnabled, member bool, level int, err error)
	GetNPMPackageDetails(repository, packageName, username string) (*NPMPackageDetails, error)
	ListNPMPackages(repository, username string, administrator, moderator bool, limit, offset int) ([]*NPMPackage, int, error)
	SearchNPMPackages(repository, query, username string, administrator, moderator bool, limit, offset int) ([]*NPMPackage, int, error)
	HasNPMMembership(repository, username string) (bool, error)
	RecordNPMPublication(pkg *NPMPackage, version *NPMVersion, tags map[string]string, username string) error
	RollbackNPMPublicationReview(repository, packageName, version string, previous *NPMPackage, previousTags map[string]string) error
	RecordNPMMirrorPublication(pkg *NPMPackage, versions []*NPMVersion, tags map[string]string) error
	UpdateNPMTarballSize(repository, tarballPath string, size int64) error
	SetNPMDistTag(repository, packageName, tag, version, actor string, expectedRevision int64) error
	DeleteNPMDistTag(repository, packageName, tag, actor string, expectedRevision int64) error
	SetNPMVersionDeprecated(repository, packageName, version, deprecated, actor string, expectedRevision int64) error
	UpdateNPMPackument(repository, packageName, actor string, expectedRevision int64, deprecations, tags map[string]string) error
	UnpublishNPMVersion(repository, packageName, version, actor string, expectedRevision int64) (string, error)
	UpdateNPMPackageDescription(repository, packageName, description, actor string) error
	SetNPMPackagePrivate(repository, packageName, actor string, private bool) error
	SetNPMPackageArchived(repository, packageName, actor string, archived bool) error
	DeleteNPMPackage(repository, packageName, actor string, expectedRevision int64) ([]string, error)
	DeleteNPMRepository(repository string) error
	ListNPMMembers(repository, packageName string) ([]*NPMMember, error)
	CreateNPMInvitations(invitations []*NPMInvitation, messages []*UserMessage) error
	RespondNPMInvitation(id, recipient, repository string, accept bool, actedAt int64) error
	ForceAddNPMMembers(repository, packageName, actor string, usernames []string, level int) error
	SetNPMMemberLevel(repository, packageName, actor, username string, level int) error
	RemoveNPMMember(repository, packageName, actor, username string) error
	RemoveNPMMembers(repository, packageName, actor string, usernames []string) error
	ApproveNPMPackageCreationReview(id, reviewer, repository, packageName, superTeamPrefix string, private bool, createdAt, decidedAt int64) (*ReviewTask, error)
}

// DockerDB manages Docker container images, tags, manifests, blobs, and repository memberships.
type DockerDB interface {
	GetDockerImage(repository, imageName string) (*DockerRepositoryImage, error)
	CreateDockerImage(repository, imageName, owner string, private bool, createdAt int64) (*DockerRepositoryImage, error)
	CreateDockerImageForTeam(repository, imageName, owner, superTeamPrefix string, private bool, createdAt int64) (*DockerRepositoryImage, error)
	GetDockerImageAccess(repository, imageName, username string) (exists, private, pushEnabled, member bool, level int, err error)
	DockerImageMemberLevels(repository, username string, imageNames []string) (map[string]int, error)
	UpdateDockerImageDescription(repository, imageName, description string) error
	UpdateDockerImageReadme(repository, imageName, readme string) error
	ListDockerImages(repository, last string, limit int) ([]*DockerRepositoryImage, error)
	SearchDockerImages(repository, query string, limit, offset int) ([]*DockerRepositoryImage, int, error)
	GetDockerImageDetails(repository, imageName string, username ...string) (*DockerImageDetails, error)
	GetDockerImageDetailsForViewer(repository, imageName, username string, moderator bool) (*DockerImageDetails, error)
	GetDockerTag(repository, imageName, tag string) (*DockerTag, error)
	ListDockerTags(repository, imageName, last string, limit int) ([]*DockerTag, error)
	ListDockerTagsForViewer(repository, imageName, last string, limit int, username string, moderator bool) ([]*DockerTag, error)
	GetDockerManifest(repository, imageName, digest string) (*DockerManifest, error)
	PutDockerManifest(manifest *DockerManifest, tag string, username string) error
	ApproveDockerPublicationReview(id, reviewer string, manifest *DockerManifest, tag string, decidedAt int64) (*ReviewTask, error)
	ApproveDockerImageCreationReview(id, reviewer, repository, imageName, superTeamPrefix string, private bool, createdAt, decidedAt int64) (*ReviewTask, error)
	CacheDockerManifest(manifest *DockerManifest, tag string) error
	DeleteDockerTag(repository, imageName, tag string) error
	DeleteDockerManifest(repository, imageName, digest string) error
	DeleteDockerImage(repository, imageName string) error
	DeleteDockerRepository(repository string) error
	RecordDockerBlob(repository, digest string, size int64) error
	RecordDockerImageBlob(repository, imageName, digest string) error
	HasDockerBlob(repository, digest string) (bool, int64, error)
	DockerImageReferencesBlob(repository, imageName, digest string) (bool, error)
	DeleteDockerBlob(repository, digest string) error
	EnsureDockerBlobMutable(repository, digest string) error
	EnsureDockerManifestMutable(repository, image, digest, tag string) error
	FilterDockerImageVersions(images []*DockerRepositoryImage, username string, moderator bool) error
	GetDockerRepositoryStats(repository string) (totalImages int64, totalTags int64, totalSize int64, err error)
	IncrementDockerPullCount(repository, imageName string) error
	BatchIncrementDockerPullCount(repository, imageName string, delta int64) error
	HasDockerMembership(repository, username string) (bool, error)
	GetDockerMemberLevel(repository, imageName, username string) (int, error)
	ListDockerMembers(repository, imageName string) ([]*DockerMember, error)
	CreateDockerInvitations(invitations []*DockerInvitation, messages []*UserMessage) error
	RespondDockerInvitation(id, recipient, repository string, accept bool, actedAt int64) error
	SetDockerMemberLevel(repository, imageName, actor, username string, level int) error
	RemoveDockerMember(repository, imageName, actor, username string) error
	RemoveDockerMembers(repository, imageName, actor string, usernames []string) error
	ForceAddDockerMembers(repository, imageName, actor string, usernames []string, level int) error
}

// ResourceLockDB manages exclusive resource locks, package deprecation flags, and path visibility.
type ResourceLockDB interface {
	IsPackageDeprecated(format, repository, packageKey string) (bool, error)
	EnsurePackageMutable(format, repository, packageKey string) error
	DeprecatePackage(format, repository, packageKey string, deprecatedAt int64) error
	SetResourceLock(lock *ResourceLock, actor, session string) error
	DeleteResourceLock(target ResourceLockTarget, source, actor, session string) error
	GetResourceLocks(target ResourceLockTarget, allVersions bool) ([]*ResourceLock, error)
	MavenDomainPathVisibility(repository, username string, moderator bool, paths []string) ([]bool, error)
	GetMavenPathLocks(repository, path string, descendants bool) ([]*ResourceLock, error)
	EnsureResourceMutable(target ResourceLockTarget, allVersions bool) error
	EnsureRepositoryResourcesMutable(repository string) error
	ResourceMetadataVisibility(format, repository, username string, moderator bool, targets []ResourceLockTarget) ([]bool, error)
}

// DownloadStatisticsDB manages recording and aggregating download traffic metrics.
type DownloadStatisticsDB interface {
	BatchIncrementDownloadStatistics(events []*DownloadStatisticDelta) error
	ResetDownloadStatistics(repository string) error
	QueryDownloadStatistics(query DownloadStatisticsQuery) (*DownloadStatisticsPage, error)
}

// TrafficQuotaDB manages user-specific traffic quota overrides.
type TrafficQuotaDB interface {
	GetTrafficQuotaUserOverride(username string) (*TrafficQuotaUserOverride, error)
	SetTrafficQuotaUserOverride(username string, override *TrafficQuotaUserOverride, updatedAt int64) error
	DeleteTrafficQuotaUserOverride(username string) error
}

// StateDB combines all specialized subsystem database interfaces into the complete application data layer.
type StateDB interface {
	NativePackageDB
	SystemDB
	TokenDB
	AccountDB
	APITokenDB
	SessionDB
	FidoDB
	AuthSecurityDB
	GPGDB
	AuditDB
	MailDB
	MessageDB
	PublicationQuotaDB
	SuperTeamDB
	TicketDB
	MavenDB
	CargoDB
	NPMDB
	DockerDB
	ResourceLockDB
	DownloadStatisticsDB
	TrafficQuotaDB
}
