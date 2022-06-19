# dock-test

Esta aplicação atende os requisitos oriundos do [teste](https://github.com/cdt-baas/desafio-dev-api-rest/blob/c066198c2041349f1e53587973931c8bc6b69062/readme.md), onde foram desenvolvidos
as persistencias de portador, conta e transações, demais features solicitadas são oriundas destes cadastros.

## Estrutura de tabelas
![img.png](.github/images/img.png)

## Arquitetura da aplicação
![architecture.png](.github/images/architecture.png)

- Nestre projeto foi utilizado o [PostgreSQL](https://www.postgresql.org/) como solução de banco de dados, o [Redis](https://redis.io/) foi escolhido para camada de cache 
e controle de concorrencia([distlock](https://redis.io/docs/reference/patterns/distributed-locks/)).
- Na parte de observabilidade a escolha foi pelo projeto open source [OpenTelemetry](https://opentelemetry.io/), que possibilita a distribuição de métricas e spans
para diferentes provedores de forma agnóstica.

### Pacotes
![img.png](./.github/images/packages.png)
 - A pasta /cmd contém o bootstrap/main da aplicação.
 - Dentro de /internal foram distribuidos os pacotes com o contexto de négocio.
   1. **accounts** -> Gestão das contas dos postadores;
   2. **api** -> Implementação dos handlers http;
   3. **balances** -> Gestão dos saldos das contas, disponibilizado pela view **transactions_balances**;
   4. **holders** -> Gestão dos postadores;
   5. **statements** -> Apresentação do extrato da conta, baseado nas transações. Feature separada do package **transactions** para prover maior autonomia de filtros;
   6. **transactions** -> Gestão das transações realizadas, como créditos, débitos e transferências entre contas;
 - Em /migrations disponibilizado todos os scripts sql (DDL) para migração do banco de dados.
 - Em /pkg estão disponíveis todos pacotes utilizados para criação da aplicação, estes que não possuem relação com o negócio.

## Como rodar o projeto?
```shell
docker-compose --env-file .env-docker  up -d
```
Após a inicialização de todos os serviços, eles estaram expostos em:
_A migração do banco de dados irá ser executada juntamente com a inicialição do docker-compose_
- dock-test-api -> [localhost:8080](http://localhost:8080)
- postgres -> [localhost:5432](postgres://dock-test:dock-test@localhost:5432/dock-test?sslmode=disable)
- redis-commander -> [localhost:8081](http://localhost:8081/) root - root
- jaeger -> [localhost:16686](http://localhost:16686/search)
- prometheus -> [localhost:9090](http://localhost:9090/graph)


## Como testar?
Sugiro utilizar o arquivo exportado do Insomnia, já possui todas os endpoints mapeados com exemplos de utilização.
[Arquivo](./Insomnia_dock-test.json)

Poré

## Curiosidades
1. Como geramos os mocks para utilização nos testes?
   - Na pasta ./scripts existe um arquivo shell onde mapeamos os arquivos/interfaces que necessita ser gerado um mock.
2. 
