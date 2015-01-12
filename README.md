#PostmanQ

PostmanQ - это высоко производительный почтовый сервер. PostmanQ забирает письма из AMQP очередей и отправляет 

##Предварительная подготовка(Mac OS X)

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

##Установка(Mac OS X)

Сначала уcтанавливаем [go](http://golang.org/doc/install). Затем устанавливаем postmanq:

    cd /tmp && mkdir postmanq && cd postmanq/
    export GOPATH=/tmp/postmanq/
    export GOBIN=/tmp/postmanq/bin/
    go get github.com/byorty/postmanq
    cd src/github.com/byorty/postmanq
    git checkout v.3
    go install cmd/postmanq.go
    cp /tmp/postmanq/bin/postmanq /usr/local/bin/ # как вариант
    
Затем берем из репозитория config.yaml и пишем свой файл с настройками. Все настройки подробно описаны в самом config.yaml.

##Использование

postmanq -f /path/to/config.yaml