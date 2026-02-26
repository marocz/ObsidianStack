// Package alerts implements the rule evaluation engine and webhook delivery
// for ObsidianStack alerting. Rules are evaluated against pipeline snapshots;
// webhooks are delivered to Teams, Slack, PagerDuty, or generic HTTP targets.
package alerts
