开发前必读

统一认证集成

人员中心

授权中心

消息中心

日程中心

待办中心

## 1、应用对接步骤

应用系统作为接口调用者对接开放平台的步骤如下：

1\. 在应用中心注册appid和secret

2\. 通过管理员授权，获取接口的调用权限

3\. 按“接口调用方法”调用接口

## 2、接口调用方法

应用系统调用开放平台接口时需传递**系统参数**以通过开放平台的鉴权。

### 2.1系统参数列表

|  | 参数 | 参数位置 | 参数类型 | 示例 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 1 | sd-appid | header | string | xgxt | 接口调用者在应用中心注册的appid |
| 2 | sd-algorithm | header | string | SHA1 | 签名方式，支持SHA1、MD5、SHA-256 |
| 3 | sd-timestamp | header | number | 1608626070885 | 当前时间戳（毫秒），服务端会检测时间戳是否在有效期内，有效期为2000s |
| 4 | sd-nonce | header | string | ibuaiVcKdpRxkhJA | 随机字符串 |
| 5 | sd-signature | header | string | EA5356DF4D359DAEC92CAF2CEDFD564AEC664E58 | 签名字符串，详见[签名算法](https://my.ruc.edu.cn/yhzt/apifile-web/static/media/sudy-sign-client.578cbf66aad3a247838c.zip) |

### 2.2 签名算法

#### 签名生成方法如下：

1.将系统参数按照参数名（不包含sd-signature）ASCII码从小到大排序（字典序），使用URL键值对的格式（即key1=value1&key2=value2…）拼接成字符串stringA，如下：

stringA="sd-algorithm=SHA1&sd-appid=xgxt&sd-nonce=ibuaiVcKdpRxkhJA&sd-timestamp=1608626070885”

2\. 将调用者app的secret拼接到stringA，key为应用的“secret”，得到stringSignTemp，如下：

stringSignTemp=stringA+"&key=49123c3d-0d98-4ef5-8128-bdc5f98b3fbd"

3\. 按所选签名方式对stringSignTemp进行签名得到sd-signature，SHA1方式如下：

sd-signature=toHex(SHA1(stringSignTemp)).toUpperCase()

#### 签名生成的注意点：

✓ 参数名ASCII码从小到大排序（字典序）

✓ 如果参数的值为空不参与签名

✓ 参数名区分大小写

✓ sd-signuature参数不参与签名

### 2.3签名传递方式

#### 两种签名参数传递方式

#### 优先级

Header > 参数，开放平台优先验证通过header传递的appid和签名。没有header的请求，才会验证url中的参数。

## 

3、签名算法调用样例【Java语言】[立即下载](https://my.ruc.edu.cn/yhzt/apifile-web/static/media/sudy-sign-client.578cbf66aad3a247838c.zip)