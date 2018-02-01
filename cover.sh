/usr/local/go/bin/go test -coverprofile c1.out\
 github.com/puppetlabs/go-parser/parser

/usr/local/go/bin/go test -coverprofile c2.out -coverpkg \
github.com/puppetlabs/go-parser/parser\
 github.com/puppetlabs/go-parser/validator

/usr/local/go/bin/go test -coverprofile c3.out -coverpkg \
github.com/puppetlabs/go-pspec/pspec,\
github.com/puppetlabs/go-evaluator/eval,\
github.com/puppetlabs/go-evaluator/impl,\
github.com/puppetlabs/go-evaluator/pcore,\
github.com/puppetlabs/go-evaluator/types,\
github.com/puppetlabs/go-parser/parser,\
github.com/puppetlabs/go-parser/validator\
 github.com/puppetlabs/go-evaluator/eval_test

/usr/local/go/bin/go test -coverprofile c4.out -coverpkg \
github.com/puppetlabs/go-pspec/pspec,\
github.com/puppetlabs/go-evaluator/eval,\
github.com/puppetlabs/go-evaluator/impl,\
github.com/puppetlabs/go-evaluator/pcore,\
github.com/puppetlabs/go-evaluator/types,\
github.com/puppetlabs/go-parser/parser,\
github.com/puppetlabs/go-parser/validator\
 github.com/puppetlabs/go-parser/parser_test

tail -n+2 c2.out >> c1.out
tail -n+2 c3.out >> c1.out
tail -n+2 c4.out >> c1.out
