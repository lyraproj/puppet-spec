/usr/local/go/bin/go test -coverprofile c1.out\
 github.com/lyraproj/puppet-parser/parser

/usr/local/go/bin/go test -coverprofile c2.out -coverpkg \
github.com/lyraproj/puppet-parser/parser\
 github.com/lyraproj/puppet-parser/validator

/usr/local/go/bin/go test -coverprofile c3.out -coverpkg \
github.com/lyraproj/puppet-spec/pspec,\
github.com/lyraproj/puppet-evaluator/eval,\
github.com/lyraproj/puppet-evaluator/impl,\
github.com/lyraproj/puppet-evaluator/pcore,\
github.com/lyraproj/puppet-evaluator/types,\
github.com/lyraproj/puppet-parser/parser,\
github.com/lyraproj/puppet-parser/validator\
 github.com/lyraproj/puppet-evaluator/eval_test

/usr/local/go/bin/go test -coverprofile c4.out -coverpkg \
github.com/lyraproj/puppet-spec/pspec,\
github.com/lyraproj/puppet-evaluator/eval,\
github.com/lyraproj/puppet-evaluator/impl,\
github.com/lyraproj/puppet-evaluator/pcore,\
github.com/lyraproj/puppet-evaluator/types,\
github.com/lyraproj/puppet-parser/parser,\
github.com/lyraproj/puppet-parser/validator\
 github.com/lyraproj/puppet-parser/parser_test

tail -n+2 c2.out >> c1.out
tail -n+2 c3.out >> c1.out
tail -n+2 c4.out >> c1.out
