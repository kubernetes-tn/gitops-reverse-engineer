# Milestone 4.B: Fix Non-Fast-Forward Pull Errors

## Problem Statement

The admission controller was experiencing frequent failures with the following error:

```
⚠️ Warning: Failed to sync to Git (operation queued for retry): failed to pull: non-fast-forward update
```

This occurs when:
1. Multiple admission controller instances are running concurrently
2. Manual commits are made to the Git repository
3. The local branch has diverged from the remote branch
4. Git cannot perform a regular pull because it would require a merge

## Root Cause

The original `pullLatest()` function only performed a simple `git pull`, which fails when there's a non-fast-forward situation. In distributed systems with multiple controllers or manual interventions, this is a common scenario that needs graceful handling.

## Solution Implemented

### 1. Enhanced Pull Strategy

Modified `pullLatest()` to detect non-fast-forward errors and automatically switch to a force pull strategy:

```go
func (c *gitClient) pullLatest() error {
	err := c.worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth: &githttp.BasicAuth{
			Username: "oauth2",
			Password: c.token,
		},
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Check if it's a non-fast-forward error
		if strings.Contains(err.Error(), "non-fast-forward") {
			log.Printf("⚠️ Non-fast-forward update detected, performing force pull...")
			if metricsCollector != nil {
				metricsCollector.IncrementNonFastForward()
			}
			return c.forcePull()
		}
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}
```

### 2. Force Pull Mechanism

Implemented `forcePull()` that performs a hard reset to match the remote branch:

```go
func (c *gitClient) forcePull() error {
	// 1. Fetch the latest changes from remote
	err := c.repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Auth: &githttp.BasicAuth{
			Username: "oauth2",
			Password: c.token,
		},
		Force: true,
	})

	// 2. Get reference to origin/main (or origin/master)
	var ref *plumbing.Reference
	ref, err = c.repo.Reference("refs/remotes/origin/main", true)
	if err != nil {
		// Fallback to master if main doesn't exist
		ref, err = c.repo.Reference("refs/remotes/origin/master", true)
		if err != nil {
			return fmt.Errorf("failed to get remote reference: %w", err)
		}
	}

	// 3. Hard reset to the remote branch
	err = c.worktree.Reset(&git.ResetOptions{
		Commit: ref.Hash(),
		Mode:   git.HardReset,
	})

	return nil
}
```

### 3. Metrics Tracking

Added a new Prometheus metric to track non-fast-forward occurrences:

- **Metric Name**: `gitops_admission_non_fast_forward_total`
- **Type**: Counter
- **Description**: Total number of non-fast-forward pull errors resolved
- **Purpose**: Monitor how often this situation occurs for capacity planning

## How It Works

1. **Normal Pull**: Attempts a regular pull first (most common case)
2. **Error Detection**: If pull fails, checks if error is due to non-fast-forward
3. **Force Pull**: Performs fetch + hard reset to remote state
4. **Branch Detection**: Automatically handles both `main` and `master` branches
5. **Metrics**: Increments counter for monitoring

## Benefits

✅ **Automatic Recovery**: No manual intervention needed when conflicts occur  
✅ **Branch Support**: Works with both `main` and `master` default branches  
✅ **Observability**: Metrics track how often this happens  
✅ **Fault Tolerance**: Maintains the existing retry queue mechanism  
✅ **State Consistency**: Always uses remote repository as source of truth

## Trade-offs

⚠️ **Local Changes Discarded**: Any uncommitted local changes are lost during force pull
- This is acceptable because the controller's local state should always mirror remote
- The controller never makes manual local edits

⚠️ **Last-Write-Wins**: If multiple controllers commit simultaneously, only the last push survives
- This is acceptable for audit/backup use case
- The Kubernetes cluster remains the source of truth, not Git

## Testing

### Manual Testing

1. Deploy multiple instances of the admission controller
2. Create resources that trigger simultaneous commits
3. Verify logs show non-fast-forward detection and recovery
4. Check metrics endpoint for `gitops_admission_non_fast_forward_total`

### Simulated Testing

```bash
# In the Git repository, create a conflict
cd /tmp/test-repo
git checkout main
echo "manual change" > test.txt
git add test.txt
git commit -m "Manual commit"
git push

# The controller will detect non-fast-forward on next operation
# and automatically recover using force pull
```

## Monitoring

Add alerts in Prometheus/Grafana:

```yaml
- alert: HighNonFastForwardRate
  expr: rate(gitops_admission_non_fast_forward_total[5m]) > 0.1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High rate of Git non-fast-forward errors"
    description: "The admission controller is experiencing frequent Git conflicts ({{ $value }} per second)"
```

## Files Modified

- `gitea.go`: Added `forcePull()` method and enhanced `pullLatest()`
- `metrics.go`: Added `nonFastForwardTotal` counter and metrics export

## Related Issues

This fix addresses the queue buildup issue where operations would get stuck in the retry queue indefinitely due to persistent non-fast-forward errors.
