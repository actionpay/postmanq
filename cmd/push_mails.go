package main

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"sync"
)

func main() {

	//	amqpURI := "amqp://admin:admin0987654321@192.168.13.130:5672/postmanq"
	amqpURI := "amqp://solomonov:123123123@localhost:5672/postmanq"
	messageCount := 1
	//	hasError := true
	hasError := false
	exchange := "postmanq"
	//	exchange := "postmanq.failure.recipient"
	envelope := "robotron@ap-ok.ru"
	recipient := "3keltw+1191enyyarlwg@sharklasers.com"
	//recipient := "asolomonoff@gmail.com"
	//	recipient := "byorty@yandex.ru""
	//	recipient := "byorty@mail.ru"
	//	recipient := "byorty@fastmail.com"
	//	recipient := "byorty@outlook.com"
	//	recipient := "byorty@qip.ru"
	//	recipient := "byorty@tut.by"
	//	recipient := "asolomonoff@yahoo.com"
	//	recipient := "byorty@nextmail.ru"
	//	recipient := "byorty@rambler.ru"
	//	recipient := "solomonov@km.ru"
	//	recipient := "byorty@zmail.ru"
	//	recipient := "byorty@meta.ua"
	//	recipient := "byorty@e1.ru"
	//	recipient := "byorty@inet.ua"
	//	recipient := "recipient": "byorty@bigmir.net"
	//	recipient := "byorty@chat.ru"

	message := `MIME-Version: 1.0
Content-Type: multipart/mixed;
	boundary="=_cb8a36ec3e182808407241c0bfcb545b";
Content-Transfer-Encoding: 7bit
Subject: =?utf-8?B?0JLQvtGB0YHRgtCw0L3QvtCy0LvQtdC90LjQtSDQv9Cw0YDQvtC70Y8g0L3QsCBhY3Rpb25wYXkucnU=?=
To: =?utf-8?B?0KHQvtC70L7QvNC+0L3QvtCyINCQ0LvQtdC60YHQtdC5?= <` + recipient + `>
From: =?utf-8?B?QWN0aW9ucGF5?= <` + envelope + `>
Return-Path: ` + envelope + `


--=_cb8a36ec3e182808407241c0bfcb545b
Content-Type: multipart/alternative;
	boundary="=_2898bd2e10dc12426ad35bd35ff604b6";


--=_2898bd2e10dc12426ad35bd35ff604b6
Content-Type: text/plain;charset="utf-8";
Content-Transfer-Encoding: quoted-printable

=d0=a7=d1=82=d0=be=d0=b1=d1=8b =d0=b2=d0=be=d1=81=d1=81=d1=82=d0=b0=d0=bd=
=d0=be=d0=b2=d0=b8=d1=82=d1=8c =d0=bf=d0=b0=d1=80=d0=be=d0=bb=d1=8c =d0=bd=
=d0=b0 =d1=81=d0=b0=d0=b9=d1=82=d0=b5 actionpay.ru, =d0=bf=d1=80=d0=be=d0=
=b9=d0=b4=d0=b8=d1=82=d0=b5 =d0=bf=d0=be =d1=81=d1=81=d1=8b=d0=bb=d0=ba=d0=
=b5:	http://actionpay.ru/ru/forget/act:restore;token:34b0c8449ad2dfbc3e6bd94=
1f70cbe69=0a=d0=98=d0=bb=d0=b8 =d0=b2=d0=b2=d0=b5=d0=b4=d0=b8=d1=82=d0=b5 =
=d0=b2=d1=80=d1=83=d1=87=d0=bd=d1=83=d1=8e =d0=ba=d0=be=d0=b4: 34b0c8449ad2d=
fbc3e6bd941f70cbe69	=d0=bf=d1=80=d0=be=d0=b9=d0=b4=d1=8f =d0=bf=d0=be =d1=
=81=d1=81=d1=8b=d0=bb=d0=ba=d0=b5:	http://actionpay.ru/ru/forget/act:restore=
=0a=d0=9d=d0=b0=d0=bf=d0=be=d0=bc=d0=b8=d0=bd=d0=b0=d0=b5=d0=bc, =d0=92=d0=
=b0=d1=88 =d0=bb=d0=be=d0=b3=d0=b8=d0=bd: solomonov

--=_2898bd2e10dc12426ad35bd35ff604b6
Content-Type: text/html;charset="utf-8";
Content-Transfer-Encoding: quoted-printable

<html>=0a<head>=0a	<meta http-equiv=3d"Content-Type" content=3d"text/html; c=
harset=3dutf-8" />=0a	<title>=d0=92=d0=be=d1=81=d1=81=d1=82=d0=b0=d0=bd=d0=
=be=d0=b2=d0=bb=d0=b5=d0=bd=d0=b8=d0=b5 =d0=bf=d0=b0=d1=80=d0=be=d0=bb=d1=
=8f =d0=bd=d0=b0 actionpay.ru</title>=0a</head>=0a<body style=3d"font: 14px =
Arial, Helvetica, sans-serif; background-color: white; color: #626262; paddi=
ng-top: 20px;">=0a<div style=3d"float: left; padding: 23px 40px;">=0a	<img a=
lt=3d"Actionpay" width=3d"145" height=3d"33" align=3d"center" src=3d"http://=
actionpay.ru/static/img/style/textlogo.png" />=0a</div>=0a<div style=3d"floa=
t: left; width: 50%;">=0a	<h1 style=3d"font-family: Arial,=e2=80=8bsans-seri=
f; font-size: 28px;">=d0=92=d0=be=d1=81=d1=81=d1=82=d0=b0=d0=bd=d0=be=d0=b2=
=d0=bb=d0=b5=d0=bd=d0=b8=d0=b5 =d0=bf=d0=b0=d1=80=d0=be=d0=bb=d1=8f =d0=bd=
=d0=b0 actionpay.ru</h1>=0a		<p>=0a		=d0=a7=d1=82=d0=be=d0=b1=d1=8b =d0=b2=
=d0=be=d1=81=d1=81=d1=82=d0=b0=d0=bd=d0=be=d0=b2=d0=b8=d1=82=d1=8c =d0=bf=
=d0=b0=d1=80=d0=be=d0=bb=d1=8c =d0=bd=d0=b0 =d1=81=d0=b0=d0=b9=d1=82=d0=b5 a=
ctionpay.ru, =d0=bf=d1=80=d0=be=d0=b9=d0=b4=d0=b8=d1=82=d0=b5 =d0=bf=d0=be =
=d1=81=d1=81=d1=8b=d0=bb=d0=ba=d0=b5:		<br />=0a		<a href=3d"http://actionpa=
y.ru/ru/forget/act:restore;token:34b0c8449ad2dfbc3e6bd941f70cbe69">http://ac=
tionpay.ru/ru/forget/act:restore;token:34b0c8449ad2dfbc3e6bd941f70cbe69</a>	=
</p>=0a	<p>=0a		=d0=98=d0=bb=d0=b8 =d0=b2=d0=b2=d0=b5=d0=b4=d0=b8=d1=82=d0=
=b5 =d0=b2=d1=80=d1=83=d1=87=d0=bd=d1=83=d1=8e =d0=ba=d0=be=d0=b4: <b>34b0c8=
449ad2dfbc3e6bd941f70cbe69</b>		<br />=0a		=d0=bf=d1=80=d0=be=d0=b9=d0=b4=
=d1=8f =d0=bf=d0=be =d1=81=d1=81=d1=8b=d0=bb=d0=ba=d0=b5:		<br />=0a		<a hre=
f=3d"http://actionpay.ru/ru/forget/act:restore">http://actionpay.ru/ru/forge=
t/act:restore</a>	</p>=0a	<p>=0a		=d0=9d=d0=b0=d0=bf=d0=be=d0=bc=d0=b8=d0=
=bd=d0=b0=d0=b5=d0=bc, =d0=92=d0=b0=d1=88 =d0=bb=d0=be=d0=b3=d0=b8=d0=bd: so=
lomonov	</p>=0a	<p><br /><b>=d0=9a=d1=80=d1=83=d0=b3=d0=bb=d0=be=d1=81=d1=
=83=d1=82=d0=be=d1=87=d0=bd=d0=b0=d1=8f =d1=81=d0=bb=d1=83=d0=b6=d0=b1=d0=
=b0 =d1=82=d0=b5=d1=85=d0=bd=d0=b8=d1=87=d0=b5=d1=81=d0=ba=d0=be=d0=b9 =d0=
=bf=d0=be=d0=b4=d0=b4=d0=b5=d1=80=d0=b6=d0=ba=d0=b8 =d0=b2=d0=b5=d0=b1=d0=
=bc=d0=b0=d1=81=d1=82=d0=b5=d1=80=d0=be=d0=b2:</b><br /><br />=0a		ICQ:	643-=
964-852<br />=0a		Skype:	actionpay24<br />=0a		E-mail:	support@actionpay.ru<=
br />=0a	</p>=0a	<p style=3d"margin-top: 50px; color: #999; font-size: 11px;=
">&copy; 2010-2014, All rights reserved. Affiliate network =c2=abActionpay=
=c2=bb</p>=0a</div>=0a</body>=0a</html>

--=_2898bd2e10dc12426ad35bd35ff604b6--

--=_cb8a36ec3e182808407241c0bfcb545b--
`

	fmt.Println("dialing ", amqpURI)
	connection, err := amqp.Dial(amqpURI)
	if err != nil {
		fmt.Errorf("Dial: %s", err)
	}
	defer connection.Close()

	fmt.Println("got Connection, getting Channel")
	channel, err := connection.Channel()
	if err != nil {
		fmt.Errorf("Channel: %s", err)
	}
	fmt.Println("got Channel")

	group := new(sync.WaitGroup)
	group.Add(messageCount)
	msg := map[string]interface{}{
		"envelope":  envelope,
		"recipient": recipient,
		"body":      message,
	}
	if hasError {
		msg["error"] = map[string]interface{}{
			"code":    551,
			"message": "unknown trololo",
		}
	}
	js, err := json.Marshal(msg)
	for i := 0; i < messageCount; i++ {
		go func() {
			if err = channel.Publish(
				exchange,
				"",    // routing to 0 or more queues
				false, // mandatory
				false, // immediate
				amqp.Publishing{
					Headers:         amqp.Table{},
					ContentType:     "text/plain",
					ContentEncoding: "",
					Body:            js,
					DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
					Priority:        0,              // 0-9
					// a bunch of application/implementation-specific fields
				},
			); err != nil {
				fmt.Errorf("Exchange Publish: %s", err)
			}
			group.Done()
		}()
	}
	group.Wait()
}
