resource "aws_s3_bucket" "s3" {
  bucket = "randomcoww-etcd-test"
}

resource "aws_iam_user" "s3" {
  name = "etcd-test"
}

resource "aws_iam_user_policy" "s3" {
  name = aws_iam_user.s3.name
  user = aws_iam_user.s3.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = "*"
        Resource = [
          "arn:aws:s3:::${aws_s3_bucket.s3.bucket}",
          "arn:aws:s3:::${aws_s3_bucket.s3.bucket}/${local.s3_path}",
          "arn:aws:s3:::${aws_s3_bucket.s3.bucket}/${local.s3_path}/*",
        ]
      }
    ]
  })
}

resource "aws_iam_access_key" "s3" {
  user = aws_iam_user.s3.name
}