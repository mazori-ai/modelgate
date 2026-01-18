import { ApolloClient, InMemoryCache, createHttpLink, ApolloLink } from '@apollo/client'
import { onError } from '@apollo/client/link/error'

const httpLink = createHttpLink({
  uri: '/graphql',
})

// Auth middleware - adds authorization header to all requests
const authMiddleware = new ApolloLink((operation, forward) => {
  const token = localStorage.getItem('authToken')

  if (token) {
    operation.setContext(({ headers = {} }) => ({
      headers: {
        ...headers,
        authorization: `Bearer ${token}`,
      },
    }))
  }

  return forward(operation)
})

const errorLink = onError(({ graphQLErrors, networkError }) => {
  if (graphQLErrors) {
    graphQLErrors.forEach(({ message }) => {
      console.error('[GraphQL Error]:', message)
      if (message.includes('unauthorized') || message.includes('Unauthorized')) {
        localStorage.removeItem('authToken')
        window.location.href = '/login'
      }
    })
  }
  if (networkError) {
    console.error('[Network Error]:', networkError)
  }
})

export const client = new ApolloClient({
  link: ApolloLink.from([errorLink, authMiddleware, httpLink]),
  cache: new InMemoryCache({
    typePolicies: {
      Query: {
        fields: {
          requestLogs: {
            keyArgs: ['filter'],
            merge(existing, incoming) {
              return incoming
            },
          },
        },
      },
    },
  }),
  defaultOptions: {
    watchQuery: {
      fetchPolicy: 'network-only',
    },
  },
})

export default client
