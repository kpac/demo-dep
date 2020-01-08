/*
Copyright The Kubepack Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	kubepackv1alpha1 "kubepack.dev/kubepack/apis/kubepack/v1alpha1"
	versioned "kubepack.dev/kubepack/client/clientset/versioned"
	internalinterfaces "kubepack.dev/kubepack/client/informers/externalversions/internalinterfaces"
	v1alpha1 "kubepack.dev/kubepack/client/listers/kubepack/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// BundleInformer provides access to a shared informer and lister for
// Bundles.
type BundleInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.BundleLister
}

type bundleInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewBundleInformer constructs a new informer for Bundle type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewBundleInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredBundleInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredBundleInformer constructs a new informer for Bundle type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredBundleInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KubepackV1alpha1().Bundles().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KubepackV1alpha1().Bundles().Watch(options)
			},
		},
		&kubepackv1alpha1.Bundle{},
		resyncPeriod,
		indexers,
	)
}

func (f *bundleInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredBundleInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *bundleInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kubepackv1alpha1.Bundle{}, f.defaultInformer)
}

func (f *bundleInformer) Lister() v1alpha1.BundleLister {
	return v1alpha1.NewBundleLister(f.Informer().GetIndexer())
}
