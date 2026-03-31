## OAuth协议集成

OAuth（Open Authorization）是一套关于授权管理的开放式网络标准协议，其核心能力是允许用户授权第三方应用，访问自身存放在其他服务提供者处的资源，全程无需将用户名、密码等核心凭证透露给第三方应用。该协议目前已在全球范围内广泛应用，主流版本为 OAuth 2.0。

OAuth 2.0 协议为用户资源授权提供了一套安全、开放且易用的标准化方案。与传统授权模式的核心区别在于：第三方应用无需获取用户的账号密码等敏感信息，即可合法申请并获得用户资源的访问授权，极大降低了用户凭证泄露的风险。

本章节将详细阐述 OAuth 2.0 协议的集成场景与实施规范，帮助第三方应用开发者遵循标准流程完成对接，最终实现统一身份认证、单点登录及全局登出的核心功能。

## 一、核心交换流程

![ 这是一张图片，ocr 内容为：第三方应用 统一身份认证 1:访问应用 请求OAUTH授权登录 响应登录界面 2:用户登录 回调应用,提供授权码CODE 3:通过授权码CODE,调用ACCESS_TOKEN接口 返回ACCESS_TOKEN 4:通过ACCESS_TOKEN获取用户信息 返回用户信息 5:创建本地会话 登录成功](https://my.ruc.edu.cn/yhzt/apifile-web/img/xhjhlc.png)

1、用户访问未登录状态的第三方应用时，应用向统一身份认证服务发起授权登录请求，强制浏览器跳转至统一身份认证登录界面。

2、用户输入用户名、密码等身份凭证并提交后，统一身份认证服务完成身份验证，将请求重定向至第三方应用，并携带授权码 code 参数。

3、第三方应用使用该授权码 code ，调用统一身份认证服务的令牌获取接口，申请获取授权令牌access\_token。

4、第三方应用在access\_token有效期内，使用该令牌调用统一身份认证服务的API接口，获取用户身份信息。

5、统一身份认证服务将用户信息返回至第三方应用后，第三方应用为用户创建本地登录会话（避免重复认证），同时开放目标业务资源的访问权限，用户正常使用应用功能。

## 二、接口说明

## 2.1 接口列表

| 接口名称 | 接口地址 | 备注 |
| --- | --- | --- |
| 获取授权码 | [https://cas.ruc.edu.cn/cas/oauth2.0/authorize](https://cas.ruc.edu.cn/cas/oauth2.0/authorize) |  |
| 获取AccessToken | [https://cas.ruc.edu.cn/cas/oauth2.0/accessToken](https://cas.ruc.edu.cn/cas/oauth2.0/accessToken) |  |
| 获取用户多账号信息 | [https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles](https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles) | scope=all时调用 |

## 2.2 接口描述

### 2.2.1 获取授权码

##### 功能说明

该接口是统一身份认证平台的核心授权入口接口，核心作用是完成用户身份验证与授权确认，当用户身份认证通过且同意授权后，接口会自动将请求重定向至第三方应用预先配置的回调地址，并在重定向 URL 中携带有效的授权码code参数，为第三方应用后续换取访问令牌access\_token提供核心凭证支撑。

##### 触发场景

用户访问第三方应用的受保护资源，第三方应用检测到用户未持有有效本地会话时，第三方应用自动构造统一认证平台「获取授权码接口」的请求参数，强制将用户浏览器请求重定向至该获取授权码接口，触发接口执行。

##### 请求URL

[https://cas.ruc.edu.cn/cas/oauth2.0/authorize](https://cas.ruc.edu.cn/cas/oauth2.0/authorize)

##### 请求方式

浏览器跳转（GET）

##### 请求参数

| 参数名 | 参数类型 | 是否必填 | 描述 |
| --- | --- | --- | --- |
| client\_id | string | 是 | 后台OAuth配置维护后分配给第三方的应用标识 |
| redirect\_uri | string | 是 | 成功授权后的授权回调地址，必须是配置时填写的主域名下的地址，建议设置为网站首页。注意：需要将url进行URLEncode编码。 |
| response\_type | string | 是 | 授权类型，此值固定为：code |
| scope | string | 是 | 授权范围，此值为：all，多个以英文逗号分割 |
| state | string | 是 | client端的状态值。用于第三方应用防止CSRF攻击，成功授权后回调时会原样带回。请务必严格按照流程检查用户与state参数状态的绑定 |

##### 请求示例

[https://cas.ruc.edu.cn/cas/oauth2.0/authorize?response\_type=code&client\_id=oauth\_demo&state=123456789](https://cas.ruc.edu.cn/cas/oauth2.0/authorize?response_type=code&client_id=oauth_demo&state=123456789) &scope=cas\_get\_userInfo&redirect\_uri=https%3A%2F%2Fclient%2Eexample%2Ecom%2Fcallback

##### 成功返回示例

HTTP Status: 302 REDIRECT

Location: [https://client.example.com/callback?code=ST-5-xxx&state=123456789](https://client.example.com/callback?code=ST-5-xxx&state=123456789)

##### 失败返回示例

失败时，登录界面在浏览器上会提示具体错误信息。

### 2.2.2 获取Access Token

##### 功能说明

该接口是 OAuth 2.0 授权体系的核心枢纽，支持第三方应用通过服务端后台请求，将一次性有效授权码code兑换为可访问用户资源的访问令牌access\_token，为后续 API 调用提供身份与权限凭证。

##### 触发场景

第三方应用获取有效授权码code后，需兑换令牌以访问用户资源时调用。

##### 请求URL

[https://cas.ruc.edu.cn/cas/oauth2.0/accessToken](https://cas.ruc.edu.cn/cas/oauth2.0/accessToken)

##### 请求方式

POST/GET

##### 请求参数

| 参数名 | 参数类型 | 是否必填 | 描述 |
| --- | --- | --- | --- |
| code | string | 是 | 授权码请求中返回的code，只能用一次 |
| client\_id | string | 是 | 后台OAuth配置维护后分配给第三方的应用标识 |
| client\_secret | string | 是 | 后台OAuth配置维护后分配给第三方的应用密钥 |
| grant\_type | string | 是 | 授权类型，此值固定为：authorization\_code |
| redirect\_uri | string | 是 | 与授权码请求时redirect\_uri参数值保持一致 |

##### 请求示例

curl --location --request POST '[https://cas.ruc.edu.cn/cas/oauth2.0/accessToken](https://cas.ruc.edu.cn/cas/oauth2.0/accessToken)' --data-urlencode 'client\_id=oauth\_demo' --data-urlencode 'client\_secret=10842d71-69a3-4bf4-98fe-bee0ed0a2bfb' --data-urlencode 'grant\_type=authorization\_code' \--data-urlencode 'redirect\_uri=https%3A%2F%2Fclient%2Eexample%2Ecom%2Fcallback'

\--data-urlencode 'code=ST-41-tIHUNtNDvicWO0pIpALb-cas-yhzt.sudytech.cn'

##### 成功返回示例

![image.png](https://my.ruc.edu.cn/yhzt/apifile-web/img/cgfhsl.png)

##### 失败返回示例

error=invalid\_request

### 2.2.3 获取用户多账号信息

##### 功能说明

此接口主要用于第三方应用使用统一身份认证登录时，从统一身份认证中获取当前登录用户的多账号信息。

##### 触发场景

第三方应用通过授权码换取access\_token后，需获取用户身份信息（如用户名、昵称、手机号）以完成本地登录会话创建，触发该接口调用。

##### 请求URL

[https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles](https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles)

##### 请求方式

GET

##### 请求参数

| 参数名 | 参数类型 | 是否必填 | 描述 |
| --- | --- | --- | --- |
| access\_token | string | 是 | 授权令牌 |

##### 请求示例

[https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles](https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles)?access\_token=xxx

##### 成功返回示例

```javascript
{ "loginName": "0100001", "name": "张勇", "gender": "男", "email": null, "phone": "", "accounts": [ { "orgName": "信息技术中心", "orgCode": "706600", "loginName": "0100001", "isMainOrg": true, "categoryName": "教职工" }, { "orgName": "信息学院", "orgCode": "101700", "loginName": "A000012", "isMainOrg": false, "categoryName": "学生" } ] }
```

##### 返回字段释义及说明

| 属性名 | 说明 |
| --- | --- |
| loginName | 登录账号（学工号） |
| name | 姓名 |
| gender | 性别 |
| email | 邮箱 |
| phone | 手机号 |
| accounts.loginName | 登录账号关联的多账号 |
| accounts.categoryName | 多账号人员类别 |
| accounts.orgCode | 多账号机构/部门机构代码 |
| accounts.orgName | 多账号机构/部门机构名称 |
| accounts.isMainOrg | 多账号所属部门是否主部门 |

##### 失败返回示例

![image.png](https://my.ruc.edu.cn/yhzt/apifile-web/img/sbfhsloauth.png)

### 2.2.5 全局退出

##### 功能说明

该接口为统一身份认证的全局登出核心接口，用于接收业务系统或用户发起的登出请求，支持单点登出（SLO）能力。接口调用后将同步销毁统一身份认证侧的用户全局会话（TGT）、所有关联业务系统的本地登录会话，同时清除用户浏览器中存储的统一身份认证会话 Cookie（CASTGC），确保用户退出后无法再通过已失效的会话访问任一关联应用。

##### 触发场景

用户主动在任一应用中点击 “退出登录” 按钮，应用调用本接口触发全局登出；管理员在统一身份认证后台强制注销用户会话，通过本接口同步所有关联应用登出状态。

##### 请求URL

[https://cas.ruc.edu.cn/cas/logout](https://cas.ruc.edu.cn/cas/logout)

##### 请求方式

浏览器跳转（GET）

##### 请求参数

| 参数名 | 参数类型 | 是否必填 | 描述 |
| --- | --- | --- | --- |
| service | string | 否 | 回调地址 |

##### 请求示例

[https://cas.ruc.edu.cn/cas/logout?service=http%3A%2F%2Foa.sudytech.com%2Findex.jsp](https://cas.ruc.edu.cn/cas/logout?service=http%3A%2F%2Foa.sudytech.com%2Findex.jsp)

##### 返回参数

无

##### 成功返回示例

统一认证注销SSO会话完成后，浏览器重定向到service指定地址。若未携带service地址时，则跳转到注销成功提示界面。