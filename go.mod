module github.com/lyraproj/puppet-spec

require (
	github.com/kisielk/gotool v1.0.0 // indirect
	github.com/lyraproj/issue v0.0.0-20190329160035-8bc10230f995
	github.com/lyraproj/pcore v0.0.0-20190408134742-7ef8f288585f
	github.com/lyraproj/puppet-evaluator v0.0.0-20190408134831-48d551aeb21e
	github.com/lyraproj/puppet-parser v0.0.0-20190408134638-04ce07bb0d8a
	github.com/lyraproj/servicesdk v0.0.0-20190408134916-985421696619
)

replace (
	github.com/lyraproj/pcore => ../pcore
	github.com/lyraproj/puppet-evaluator => ../puppet-evaluator
	github.com/lyraproj/servicesdk => ../servicesdk
)
