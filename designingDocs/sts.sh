#!/usr/bin/env bash
set -euo pipefail

# ====== 你的配置 ======
AK="HPUA7GKRJ3UZCI68D3JK"
SK="bhENrYEPEumlTfPMdBEvjtzEgxb0QGeikmp7AZMe"

HOST="iam.myhuaweicloud.com"
ENDPOINT="https://${HOST}"

# 实际请求路径：不带 /
REQUEST_URI="/v3.0/OS-CREDENTIAL/securitytokens"

# 签名用路径：带 /
CANONICAL_URI="/v3.0/OS-CREDENTIAL/securitytokens/"

CONTENT_TYPE="application/json;charset=utf8"
X_SDK_DATE="$(date -u +%Y%m%dT%H%M%SZ)"

# ====== STS 请求体 ======
cat > sts-body.json <<'JSON'
{
  "auth": {
    "identity": {
      "methods": ["token"],
      "token": {
        "duration_seconds": 1800
      },
      "policy": {
        "Version": "1.1",
        "Statement": [
          {
            "Effect": "Allow",
            "Action": [
              "obs:object:PutObject"
            ],
            "Resource": [
              "obs:*:*:object:skilljudge-8af9/*"
            ]
          }
        ]
      }
    }
  }
}
JSON

# ====== 工具函数 ======
sha256_file() {
  openssl dgst -sha256 -binary "$1" | xxd -p -c 256
}

sha256_text() {
  printf '%s' "$1" | openssl dgst -sha256 -binary | xxd -p -c 256
}

hmac_sha256_hex() {
  local key="$1"
  local data="$2"
  printf '%s' "$data" | openssl dgst -sha256 -mac HMAC -macopt "key:${key}" -binary | xxd -p -c 256
}

# ====== 计算 Body Hash ======
BODY_HASH="$(sha256_file sts-body.json)"

# ====== 构造 Canonical Headers ======
CANONICAL_HEADERS=$'content-type:'"${CONTENT_TYPE}"$'\n''host:'"${HOST}"$'\n''x-sdk-date:'"${X_SDK_DATE}"$'\n'
SIGNED_HEADERS="content-type;host;x-sdk-date"

# ====== 构造 Canonical Request ======
# 格式必须严格是：
# METHOD \n
# CanonicalURI \n
# CanonicalQueryString(空) \n
# CanonicalHeaders \n
# SignedHeaders \n
# HexEncode(Hash(RequestPayload))
CANONICAL_REQUEST=$'POST\n'"${CANONICAL_URI}"$'\n\n'"${CANONICAL_HEADERS}"$'\n'"${SIGNED_HEADERS}"$'\n'"${BODY_HASH}"

HASHED_CANONICAL_REQUEST="$(sha256_text "${CANONICAL_REQUEST}")"

# ====== 构造 StringToSign ======
STRING_TO_SIGN=$'SDK-HMAC-SHA256\n'"${X_SDK_DATE}"$'\n'"${HASHED_CANONICAL_REQUEST}"

# ====== 计算签名 ======
SIGNATURE="$(hmac_sha256_hex "${SK}" "${STRING_TO_SIGN}")"

AUTHORIZATION="SDK-HMAC-SHA256 Access=${AK}, SignedHeaders=${SIGNED_HEADERS}, Signature=${SIGNATURE}"

# ====== 打印调试信息 ======
echo "=== Canonical Request ==="
printf '%s\n' "${CANONICAL_REQUEST}"
echo

echo "=== String To Sign ==="
printf '%s\n' "${STRING_TO_SIGN}"
echo

echo "=== Authorization ==="
printf '%s\n' "${AUTHORIZATION}"
echo

echo "=== Sending Request ==="

# ====== 发请求 ======
curl -v "${ENDPOINT}${REQUEST_URI}" \
  -H "Content-Type: ${CONTENT_TYPE}" \
  -H "Host: ${HOST}" \
  -H "X-Sdk-Date: ${X_SDK_DATE}" \
  -H "Authorization: ${AUTHORIZATION}" \
  --data-binary @sts-body.json
