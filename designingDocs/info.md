# OBS 相关信息
skilljudge
Ednpointobs.cn-east-3.myhuaweicloud.com
skilljudge.obs.cn-east-3.myhuaweicloud.com

AK:HPUANB4LLQ7LC5QK9DLX
SK:YXu5c5zzZy2ebwKstB9LxXB2bZQloJKvbctqmOuF
 
# bash脚本返回结果


 (base) jason@JasondeMacBook-Air pycode % ./sts.sh
=== Canonical Request ===
POST
/v3.0/OS-CREDENTIAL/securitytokens/

content-type:application/json;charset=utf8
host:iam.myhuaweicloud.com
x-sdk-date:20260324T132153Z

content-type;host;x-sdk-date
9b67dc0081ee0c1987b5807e869f30c6a15d02b270c07c6c8d09ae947d325b3b

=== String To Sign ===
SDK-HMAC-SHA256
20260324T132153Z
5676978109d9ae4514e97e0aa2479898c136bc4b6dfb2244efcd85861265e9f7

=== Authorization ===
SDK-HMAC-SHA256 Access=HPUANB4LLQ7LC5QK9DLX, SignedHeaders=content-type;host;x-sdk-date, Signature=f85a1fd75d5c630c9cb5b6e670021975fac6544ae20291d96f3d57bd635daa9e

=== Sending Request ===
* Host iam.myhuaweicloud.com:443 was resolved.
* IPv6: (none)
* IPv4: 120.46.246.26, 120.46.247.26
*   Trying 120.46.246.26:443...
* Connected to iam.myhuaweicloud.com (120.46.246.26) port 443
* ALPN: curl offers h2,http/1.1
* (304) (OUT), TLS handshake, Client hello (1):
*  CAfile: /etc/ssl/cert.pem
*  CApath: none
* (304) (IN), TLS handshake, Server hello (2):
* TLSv1.2 (IN), TLS handshake, Certificate (11):
* TLSv1.2 (IN), TLS handshake, Server key exchange (12):
* TLSv1.2 (IN), TLS handshake, Server finished (14):
* TLSv1.2 (OUT), TLS handshake, Client key exchange (16):
* TLSv1.2 (OUT), TLS change cipher, Change cipher spec (1):
* TLSv1.2 (OUT), TLS handshake, Finished (20):
* TLSv1.2 (IN), TLS change cipher, Change cipher spec (1):
* TLSv1.2 (IN), TLS handshake, Finished (20):
* SSL connection using TLSv1.2 / ECDHE-RSA-AES128-GCM-SHA256 / [blank] / UNDEF
* ALPN: server accepted http/1.1
* Server certificate:
*  subject: C=CN; ST=Jiangsu; L=Nanjing; O=Huawei Software Technologies Co., Ltd.; CN=*.myhuaweicloud.com
*  start date: Nov 19 09:46:03 2025 GMT
*  expire date: Dec 21 09:46:02 2026 GMT
*  subjectAltName: host "iam.myhuaweicloud.com" matched cert's "*.myhuaweicloud.com"
*  issuer: C=BE; O=GlobalSign nv-sa; CN=GlobalSign RSA OV SSL CA 2018
*  SSL certificate verify ok.
* using HTTP/1.x
> POST /v3.0/OS-CREDENTIAL/securitytokens HTTP/1.1
> Host: iam.myhuaweicloud.com
> User-Agent: curl/8.7.1
> Accept: */*
> Content-Type: application/json;charset=utf8
> X-Sdk-Date: 20260324T132153Z
> Authorization: SDK-HMAC-SHA256 Access=HPUANB4LLQ7LC5QK9DLX, SignedHeaders=content-type;host;x-sdk-date, Signature=f85a1fd75d5c630c9cb5b6e670021975fac6544ae20291d96f3d57bd635daa9e
> Content-Length: 430
> 
* upload completely sent off: 430 bytes
< HTTP/1.1 201 
< Server: CloudWAF
< Date: Tue, 24 Mar 2026 13:21:53 GMT
< Content-Type: application/json; charset=UTF-8
< Content-Length: 1118
< Connection: keep-alive
< Set-Cookie: HWWAFSESID=dbe7d1f9dfdb485c54b; path=/
< Set-Cookie: HWWAFSESTIME=1774358510688; path=/
< X-IAM-Trace-Id: token_cn-north-4_null_37c3199f7e6f421404d40dd82cb56b80
< X-Request-Id: 37c3199f7e6f421404d40dd82cb56b80
< Strict-Transport-Security: max-age=31536000; includeSubdomains;
< X-Frame-Options: SAMEORIGIN
< X-Content-Type-Options: nosniff
< X-Download-Options: noopen
< X-XSS-Protection: 1; mode=block;
< 
* Connection #0 to host iam.myhuaweicloud.com left intact
{"credential":{"access":"HST3WJXTJW50P58XMJD6","expires_at":"2026-03-24T13:51:53.741000Z","secret":"2MBBNruZ9JtzJnAWtNf1eqK5SreY4PUGSpV1B26F","securitytoken":"ggpjbi1ub3J0aC00UDh7ImFjY2VzcyI6IkhTVDNXSlhUSlc1MFA1OFhNSkQ2IiwiaXNzdWVkX2F0IjoxNzc0MzU4NTEzNzQxLCJtZXRob2RzIjpbInRva2VuIl0sInBvbGljeSI6eyJWZXJzaW9uIjoiMS4xIiwiU3RhdGVtZW50IjpbeyJBY3Rpb24iOlsib2JzOm9iamVjdDpQdXRPYmplY3QiXSwiUmVzb3VyY2UiOlsib2JzOio6KjpvYmplY3Q6c2tpbGx1ZGdlLyoiXSwiRWZmZWN0IjoiQWxsb3cifV19LCJyb2xlIjpbXSwicm9sZXRhZ2VzIjpbXSwidGltZW91dF9hdCI6MTc3NDM2MDMxMzc0MSwidXNlciI6eyJkb21haW4iOnsiaWQiOiJlMjA1OWY2ZWExY2M0ZTdjYTczNjUzODg0YjUwNmQ4NyIsIm5hbWUiOiJobF9naW90dG8ifSwiaWQiOiI5MzhkMmU5YjE5MTg0NmEyODA5MzQ1Zjc4NDc2NTA4NSIsIm5hbWUiOiJobF9naW90dG8iLCJwYXNzd29yZF9leHBpcmVzX2F0IjoiIiwidXNlcl90eXBlIjoxN319Zxxvpfn8AA9hh58uqr5dqrOir_-E_z1Gu4-pZ4XzUAqFiuS2A-WpHEZbzrf9rJLhFogVWXX4oYxz3oAV0wOAlGWqM4AyyY6Copo6HIRB72LFiax03W18oOqMpRt5oD0XP9YyJ9pMEDUc7stBTeBI_yyJt4F0VTOgeW9MGnWpSdjD7PqAkgy8cagUP-q8PKfe1mXtCda2SWvJJQVC4t2AYsUGlM6Q7NxE6zoAb9L1KAUGME82CHUlK-TbqtfNVxfvYYPrReoSXFnCR3sPf3vx3YLGmb6SrtGmgxH7iGH1PP1PlZZOgqaUCpv_oy2DS856MAbFqeLn_Fh-n8rgnkjuMw=="}}%                                                                                                                            