UPDATE demands
SET status = 'active',
    review_reason = NULL,
    reviewed_by_admin_id = NULL,
    reviewed_at = NULL,
    updated_at = now(),
    version = version + 1
WHERE status IN ('pending_review', 'changes_requested');
