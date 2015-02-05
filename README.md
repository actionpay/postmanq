#PostmanQ

PostmanQ - это высоко производительный почтовый сервер(MTA). 

Для работы PostmanQ потребуется AMQP-сервер, в котором будут хранится письма. 

PostmanQ разбирает одну или несколько очередей с письмами и отправляет письма по SMTP сторонним почтовым сервисам.

##Возможности

1. PostmanQ может работать с несколькими AMQP-серверами и очередями каждого из серверов.
2. PostmanQ умеет работать через TLS соединение.
3. PostmanQ проверяет заголовки письма, необходимые для создания DKIM. Если нет необходимого заголовка, тогда создается заголовок со значением по умолчанию.
4. PostmanQ подписывает DKIM для каждого письма.
5. PostmanQ следит за количеством отправленных писем почтовому сервису.
6. PostmanQ попробует отослать письмо попозже, если возникла сетевая ошибка, письмо попало в [серый список](http://ru.wikipedia.org/wiki/%D0%A1%D0%B5%D1%80%D1%8B%D0%B9_%D1%81%D0%BF%D0%B8%D1%81%D0%BE%D0%BA) или количество отправленных писем почтовому сервису уже максимально.
7. PostmanQ положит в отдельную очередь письма, которые не удалось отправить из-за 5ХХ ошибки

##Как это работает?

1. Нам потребуется AMQP-сервер, например [RabbitMQ](https://www.rabbitmq.com), и [go](http://golang.org/) для компиляции PostmanQ.
2. Выполняем предварительную подготовку и установку PostmanQ. Инструкции описаны ниже.
3. Запускаем RabbitMQ и PostmanQ.
4. Создаем в RabbitMQ одну или несколько очередей.
5. Кладем в очередь письмо. Письмо должно иметь json формат и иметь вид
    
        {
            "envelope": "sender@mail.foo",
            "recipient": "recipient@mail.foo",
            "body": "письмо с заголовками и содержимым в base64"
        }
    
6. PostmanQ забирает письмо из очереди.
7. Проверяет ограничение на количество отправленных писем для почтового сервиса.
8. Открывает TLS или обычное соединение.
9. Проверяет заголовки письма, необходимые для создания DKIM.
10. Создает DKIM.
11. Отправляет письмо стороннему почтовому сервису.
12. Если произошла сетевая ошибка, то письмо перекладывается в одну из очередей для повторной отправки.
13. Если произошла 5ХХ ошибка, то письмо перекладывается в очередь с проблемными письмами, повторная отправка не производится.

##Предварительная подготовка

В начале создаем пару ключей:

    openssl genrsa -out private.key 1024               # путь до этого ключа нужно будет указать в настройках postmanq
    openssl rsa -pubout -in private.key out public.key # этот ключ нужно будет указать в DNS
    
Закрытый ключ будет использоваться для подписи DKIM. 

Публичный ключ необходимо указать в DNS записи для того, чтобы сторонние почтовые сервисы могли валидировать DKIM наших писем.

Затем необходимо создать подписанный сертификат. Он будет использоваться для создания TLS соединений к удаленным почтовым сервисами.

    cd /some/path
    /System/Library/OpenSSL/misc/CA.pl -newca                                                # создаем центр авторизации
    echo "unique_subject = no" > someNameCA/index.txt.attr                                   # 
    openssl req -new -x509 -key private.key -out cert.pem                                    # создаем неподписанный сертификат
    cat cert.pem private.key | openssl x509 -x509toreq -signkey private.key -out certreq.csr # создаем CSR
    openssl ca -in certreq.csr -out cert.pem                                                 # создаем подписанный сертификат
    rm certreq.csr
    
Далее добавляем DKIM и SPF записи в DNS:
    
    _domainkey.example.com. TXT "t=s; o=~;"
    selector._domainkey.example.com. 3600 IN TXT "k=rsa\; t=s\; p=содержимое public.key" 
    example.com. IN TXT "v=spf1 +a +mx ~all"
    
Теперь наши письма не будут попадать в спам. 

Selector-ом может быть любым словом на латинице. Значение selector-а необходимо указать в настройках postmanq в поле dkimSelector.

Затем устанавливаем AMQP-сервер из [MacPorts](http://www.macports.org/), например [RabbitMQ](https://www.rabbitmq.com).

    sudo port install rabbitmq-server
    
Теперь все готово для установки PostmanQ.

##Установка

Сначала уcтанавливаем [go](http://golang.org/doc/install). Затем устанавливаем postmanq:

    cd /tmp && mkdir postmanq && cd postmanq/
    export GOPATH=/tmp/postmanq/
    export GOBIN=/tmp/postmanq/bin/
    go get github.com/AdOnWeb/postmanq
    cd src/github.com/AdOnWeb/postmanq
    git checkout v.3
    go install cmd/postmanq.go
    cp /tmp/postmanq/bin/postmanq /usr/local/bin/ # как вариант
    
Затем берем из репозитория config.yaml и пишем свой файл с настройками. Все настройки подробно описаны в самом config.yaml.

##Использование

    sudo rabbitmq-server -detached
    postmanq -f /path/to/config.yaml