-- name: ReportByNonce :one
SELECT * FROM abuse_reports WHERE reporter=@reporter AND nonce=@nonce;
-- name: CreateReport :execlastid
INSERT INTO abuse_reports(reporter,kind,context,target,category,details,nonce,status,created_at,updated_at,review_note) VALUES(@reporter,@kind,@context,@target,@category,@details,@nonce,'pending',@now,@now,'');
-- name: GetReport :one
SELECT * FROM abuse_reports WHERE id=@id;
-- name: ListReports :many
SELECT * FROM abuse_reports WHERE (CAST(@status AS TEXT)='' OR status=@status) ORDER BY id DESC LIMIT @count;
-- name: UpdateReportReview :exec
UPDATE abuse_reports SET status=@status,review_note=@note,updated_at=@now WHERE id=@id;
-- name: AddReportReview :exec
INSERT INTO abuse_report_reviews(report_id,status,note,created_at) VALUES(@report_id,@status,@note,@now);
-- name: ListReportReviews :many
SELECT * FROM abuse_report_reviews WHERE report_id=@id ORDER BY id;
-- name: OperatorReviewByNonce :one
SELECT kind,item_id,status,note FROM operator_reviews WHERE actor=@actor AND nonce=@nonce;
-- name: SupportItemState :one
SELECT state FROM service_entries WHERE id=@id AND kind='support';
-- name: UpdateSupportItemState :exec
UPDATE service_entries SET state=@state WHERE id=@id;
-- name: AddOperatorReview :exec
INSERT INTO operator_reviews(kind,item_id,actor,status,note,nonce,created_at) VALUES(@kind,@item_id,@actor,@status,@note,@nonce,@now);
-- name: ListOperatorReviews :many
SELECT id,kind,item_id,actor,status,note,created_at FROM operator_reviews WHERE kind=@kind AND item_id=@id ORDER BY id;
-- name: ListOperatorReports :many
SELECT * FROM abuse_reports WHERE (CAST(@before_id AS INTEGER)=0 OR id<@before_id) ORDER BY id DESC LIMIT 100;
-- name: ListOperatorSupport :many
SELECT * FROM service_entries WHERE kind='support' AND (CAST(@before_id AS INTEGER)=0 OR id<@before_id) ORDER BY id DESC LIMIT 100;
